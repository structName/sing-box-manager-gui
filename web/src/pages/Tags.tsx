import { useEffect, useState, useCallback } from 'react';
import {
  Card,
  CardBody,
  CardHeader,
  Button,
  Input,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
  Chip,
  Select,
  SelectItem,
  Switch,
  Spinner,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Tabs,
  Tab,
  Tooltip,
  Accordion,
  AccordionItem,
} from '@nextui-org/react';
import {
  Tag as TagIcon,
  Plus,
  Pencil,
  Trash2,
  Play,
  Filter,
} from 'lucide-react';
import { tagApi } from '../api';
import { toast } from '../components/Toast';

// LOAD_REST_FROM_FILE_MARKER
