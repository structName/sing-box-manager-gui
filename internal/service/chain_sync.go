package service

import (
	"github.com/xiaobei/singbox-manager/internal/storage"
)

// ChainSyncService 链路节点同步服务
type ChainSyncService struct {
	store *storage.JSONStore
}

// NewChainSyncService 创建链路同步服务
func NewChainSyncService(store *storage.JSONStore) *ChainSyncService {
	return &ChainSyncService{store: store}
}

// SyncChainNodes 同步所有链路的节点副本
// 当订阅刷新后调用，清理已失效的节点引用
func (s *ChainSyncService) SyncChainNodes() error {
	chains := s.store.GetProxyChains()
	allNodes := s.store.GetAllNodes()

	// 构建当前有效节点 Tag 集合
	validNodeTags := make(map[string]storage.Node)
	for _, node := range allNodes {
		validNodeTags[node.Tag] = node
	}

	for _, chain := range chains {
		updated := false
		validChainNodes := make([]storage.ChainNode, 0, len(chain.ChainNodes))
		validNodes := make([]string, 0, len(chain.Nodes))

		// 检查每个链路节点是否仍然有效
		for _, chainNode := range chain.ChainNodes {
			if storage.IsChainCountryNodeTag(chainNode.OriginalTag) {
				chainNode.Source = storage.GetChainCountryNodeSource(storage.ParseChainCountryNodeCode(chainNode.OriginalTag))
				validChainNodes = append(validChainNodes, chainNode)
				validNodes = append(validNodes, chainNode.OriginalTag)
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
			if storage.IsChainCountryNodeTag(chainNode.OriginalTag) {
				chainNode.Source = storage.GetChainCountryNodeSource(storage.ParseChainCountryNodeCode(chainNode.OriginalTag))
				validChainNodes = append(validChainNodes, chainNode)
				validNodes = append(validNodes, chainNode.OriginalTag)
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
		source := ""
		if storage.IsChainCountryNodeTag(tag) {
			source = storage.GetChainCountryNodeSource(storage.ParseChainCountryNodeCode(tag))
		} else {
			source = nodeMap[tag].Source
		}
		newChainNodes = append(newChainNodes, storage.ChainNode{
			OriginalTag: tag,
			CopyTag:     storage.GenerateChainNodeCopyTag(chain.Name, tag),
			Source:      source,
		})
	}

	chain.ChainNodes = newChainNodes
	return s.store.UpdateProxyChain(*chain)
}
