package service

import (
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// ChainSyncService 链路节点同步服务
type ChainSyncService struct {
	store *storage.JSONStore
}

// NewChainSyncService 创建链路同步服务
func NewChainSyncService(store *storage.JSONStore) *ChainSyncService {
	return &ChainSyncService{store: store}
}

// validNodeTagsForChainSync 构建链路同步用的有效节点集合。
// 包含 GetAllNodes()（启用的订阅/手动节点），并额外保留已禁用的手动节点 Tag，
// 避免“仅禁用”导致链路引用被误剪枝；真正删除后节点会从 ManualNodes 消失再被剪枝。
func (s *ChainSyncService) validNodeTagsForChainSync() map[string]storage.Node {
	validNodeTags := make(map[string]storage.Node)
	for _, node := range s.store.GetAllNodes() {
		validNodeTags[node.Tag] = node
	}
	for _, mn := range s.store.GetManualNodes() {
		tag := strings.TrimSpace(mn.Node.Tag)
		if tag == "" {
			continue
		}
		if _, exists := validNodeTags[tag]; exists {
			continue
		}
		node := mn.Node
		node.Source = "manual"
		if node.SourceName == "" {
			node.SourceName = "手动添加"
		}
		validNodeTags[tag] = node
	}
	return validNodeTags
}

// SyncChainNodes 同步所有链路的节点副本
// 当订阅刷新后调用，清理已失效的节点引用
func (s *ChainSyncService) SyncChainNodes() error {
	chains := s.store.GetProxyChains()
	validNodeTags := s.validNodeTagsForChainSync()

	for _, chain := range chains {
		updated := false
		validChainNodes := make([]storage.ChainNode, 0, len(chain.ChainNodes))
		validNodes := make([]string, 0, len(chain.Nodes))

		// 检查每个链路节点是否仍然有效
		for _, chainNode := range chain.ChainNodes {
			if specialNode, ok := storage.ChainSpecialNodeMetadata(chain.Name, chainNode.OriginalTag); ok {
				validChainNodes = append(validChainNodes, specialNode)
				validNodes = append(validNodes, specialNode.OriginalTag)
			} else if node, exists := validNodeTags[chainNode.OriginalTag]; exists {
				// 节点仍然存在，保留并更新来源信息
				chainNode.Source = node.Source
				validChainNodes = append(validChainNodes, chainNode)
				validNodes = append(validNodes, chainNode.OriginalTag)
			} else {
				// 节点已被删除
				updated = true
			}
		}

		// 如果有节点被移除，更新链路
		if updated {
			chain.ChainNodes = validChainNodes
			chain.Nodes = validNodes

			// 如果链路少于2个节点，可以选择禁用或保留
			// 这里选择保留，用户可以手动处理
			if err := s.store.UpdateProxyChain(chain); err != nil {
				return err
			}
		}
	}

	return nil
}

// SyncChainNodesForSubscription 同步特定订阅相关的链路
// 当单个订阅刷新后调用
func (s *ChainSyncService) SyncChainNodesForSubscription(subID string) error {
	// 获取订阅的当前节点
	sub := s.store.GetSubscription(subID)
	if sub == nil {
		return nil
	}

	// 构建订阅节点 Tag 集合
	subNodeTags := make(map[string]bool)
	for _, node := range sub.Nodes {
		subNodeTags[node.Tag] = true
	}

	chains := s.store.GetProxyChains()

	for _, chain := range chains {
		updated := false
		validChainNodes := make([]storage.ChainNode, 0, len(chain.ChainNodes))
		validNodes := make([]string, 0, len(chain.Nodes))

		for _, chainNode := range chain.ChainNodes {
			if specialNode, ok := storage.ChainSpecialNodeMetadata(chain.Name, chainNode.OriginalTag); ok {
				validChainNodes = append(validChainNodes, specialNode)
				validNodes = append(validNodes, specialNode.OriginalTag)
				continue
			}

			// 只检查来自此订阅的节点
			if chainNode.Source == subID {
				if subNodeTags[chainNode.OriginalTag] {
					// 节点仍然存在
					validChainNodes = append(validChainNodes, chainNode)
					validNodes = append(validNodes, chainNode.OriginalTag)
				} else {
					// 节点已被删除
					updated = true
				}
			} else {
				// 非此订阅的节点，保留
				validChainNodes = append(validChainNodes, chainNode)
				validNodes = append(validNodes, chainNode.OriginalTag)
			}
		}

		if updated {
			chain.ChainNodes = validChainNodes
			chain.Nodes = validNodes
			if err := s.store.UpdateProxyChain(chain); err != nil {
				return err
			}
		}
	}

	return nil
}

// RetargetNodeTag 将所有链路中的节点引用从 oldTag 改写为 newTag。
// 跳过特殊节点（country/auto/tor）。用于手动节点重命名后级联更新链路。
func (s *ChainSyncService) RetargetNodeTag(oldTag, newTag string) error {
	oldTag = strings.TrimSpace(oldTag)
	newTag = strings.TrimSpace(newTag)
	if oldTag == "" || newTag == "" || oldTag == newTag {
		return nil
	}
	// 特殊节点 Tag 不应作为普通节点被改写
	if _, ok := storage.ChainSpecialNodeMetadata("", oldTag); ok {
		return nil
	}

	allNodes := s.store.GetAllNodes()
	nodeMap := make(map[string]storage.Node, len(allNodes))
	for _, n := range allNodes {
		nodeMap[n.Tag] = n
	}
	// 禁用手动节点也可能被改名，补充来源信息
	for _, mn := range s.store.GetManualNodes() {
		tag := strings.TrimSpace(mn.Node.Tag)
		if tag == "" {
			continue
		}
		if _, exists := nodeMap[tag]; exists {
			continue
		}
		node := mn.Node
		node.Source = "manual"
		nodeMap[tag] = node
	}

	chains := s.store.GetProxyChains()
	for _, chain := range chains {
		updated := false

		for i, tag := range chain.Nodes {
			if tag == oldTag {
				chain.Nodes[i] = newTag
				updated = true
			}
		}

		if len(chain.ChainNodes) > 0 {
			newChainNodes := make([]storage.ChainNode, 0, len(chain.ChainNodes))
			for _, chainNode := range chain.ChainNodes {
				tag := chainNode.OriginalTag
				if tag == oldTag {
					tag = newTag
					updated = true
				}
				if specialNode, ok := storage.ChainSpecialNodeMetadata(chain.Name, tag); ok {
					newChainNodes = append(newChainNodes, specialNode)
					continue
				}
				source := chainNode.Source
				if node, exists := nodeMap[tag]; exists && node.Source != "" {
					source = node.Source
				} else if tag == newTag && (source == "" || chainNode.OriginalTag == oldTag) {
					if node, exists := nodeMap[newTag]; exists && node.Source != "" {
						source = node.Source
					} else if source == "" {
						source = "manual"
					}
				}
				newChainNodes = append(newChainNodes, storage.ChainNode{
					OriginalTag: tag,
					CopyTag:     storage.GenerateChainNodeCopyTag(chain.Name, tag),
					Source:      source,
				})
			}
			chain.ChainNodes = newChainNodes
		} else if updated {
			// Nodes 已改写但无 ChainNodes：按 RegenerateChainNodes 模式补齐
			newChainNodes := make([]storage.ChainNode, 0, len(chain.Nodes))
			for _, tag := range chain.Nodes {
				if specialNode, ok := storage.ChainSpecialNodeMetadata(chain.Name, tag); ok {
					newChainNodes = append(newChainNodes, specialNode)
					continue
				}
				source := ""
				if node, exists := nodeMap[tag]; exists {
					source = node.Source
				}
				newChainNodes = append(newChainNodes, storage.ChainNode{
					OriginalTag: tag,
					CopyTag:     storage.GenerateChainNodeCopyTag(chain.Name, tag),
					Source:      source,
				})
			}
			chain.ChainNodes = newChainNodes
		}

		if updated {
			if err := s.store.UpdateProxyChain(chain); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemoveNodeTag 从所有链路中移除指定节点 Tag 的引用（真正删除节点时使用）。
// 跳过特殊节点 Tag。
func (s *ChainSyncService) RemoveNodeTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}
	if _, ok := storage.ChainSpecialNodeMetadata("", tag); ok {
		return nil
	}

	chains := s.store.GetProxyChains()
	for _, chain := range chains {
		updated := false
		validNodes := make([]string, 0, len(chain.Nodes))
		for _, nodeTag := range chain.Nodes {
			if nodeTag == tag {
				updated = true
				continue
			}
			validNodes = append(validNodes, nodeTag)
		}

		validChainNodes := make([]storage.ChainNode, 0, len(chain.ChainNodes))
		for _, chainNode := range chain.ChainNodes {
			if chainNode.OriginalTag == tag {
				updated = true
				continue
			}
			validChainNodes = append(validChainNodes, chainNode)
		}

		if updated {
			chain.Nodes = validNodes
			chain.ChainNodes = validChainNodes
			if err := s.store.UpdateProxyChain(chain); err != nil {
				return err
			}
		}
	}
	return nil
}

// RegenerateChainNodes 重新生成链路的 ChainNodes
// 用于链路名称变更后更新副本 Tag
func (s *ChainSyncService) RegenerateChainNodes(chainID string) error {
	chain := s.store.GetProxyChain(chainID)
	if chain == nil {
		return nil
	}

	allNodes := s.store.GetAllNodes()
	nodeMap := make(map[string]storage.Node)
	for _, n := range allNodes {
		nodeMap[n.Tag] = n
	}

	newChainNodes := make([]storage.ChainNode, 0, len(chain.Nodes))
	for _, tag := range chain.Nodes {
		if specialNode, ok := storage.ChainSpecialNodeMetadata(chain.Name, tag); ok {
			newChainNodes = append(newChainNodes, specialNode)
			continue
		}

		source := ""
		source = nodeMap[tag].Source
		newChainNodes = append(newChainNodes, storage.ChainNode{
			OriginalTag: tag,
			CopyTag:     storage.GenerateChainNodeCopyTag(chain.Name, tag),
			Source:      source,
		})
	}

	chain.ChainNodes = newChainNodes
	return s.store.UpdateProxyChain(*chain)
}
