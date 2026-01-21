import { useState } from 'react';
import { ChevronRight, ChevronDown, Copy } from 'lucide-react';
import { CopyButton } from './copy-button';

interface TreeNode {
  id: string;
  label: string;
  value?: string;
  children?: TreeNode[];
}

interface TreeViewProps {
  data: TreeNode[];
}

function TreeItem({ node, level = 0 }: { node: TreeNode; level?: number }) {
  const [expanded, setExpanded] = useState(level < 2);
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div>
      <div
        className={`flex items-center gap-2 py-1.5 px-2 rounded-[6px] hover:bg-muted/50 transition-colors ${
          hasChildren ? 'cursor-pointer' : ''
        }`}
        style={{ paddingLeft: `${level * 20 + 8}px` }}
        onClick={() => hasChildren && setExpanded(!expanded)}
      >
        {hasChildren && (
          <button className="p-0 text-muted-foreground">
            {expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          </button>
        )}
        {!hasChildren && <div className="w-4" />}
        
        <span className="text-sm font-medium text-foreground">{node.label}</span>
        
        {node.value && (
          <>
            <code className="flex-1 text-xs font-mono text-muted-foreground truncate">
              {node.value}
            </code>
            <CopyButton text={node.value} />
          </>
        )}
      </div>
      
      {hasChildren && expanded && (
        <div>
          {node.children!.map(child => (
            <TreeItem key={child.id} node={child} level={level + 1} />
          ))}
        </div>
      )}
    </div>
  );
}

export function TreeView({ data }: TreeViewProps) {
  return (
    <div className="bg-card border border-border rounded-[12px] p-2">
      {data.map(node => (
        <TreeItem key={node.id} node={node} />
      ))}
    </div>
  );
}