import { X } from 'lucide-react';
import { CodeBlock } from './code-block';

interface InspectorDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  data: any;
}

export function InspectorDrawer({ isOpen, onClose, title, data }: InspectorDrawerProps) {
  if (!isOpen) return null;

  return (
    <>
      <div
        className="fixed inset-0 bg-background/80 backdrop-blur-sm z-40"
        onClick={onClose}
      />
      <div className="fixed right-0 top-0 bottom-0 w-[480px] bg-card border-l border-border z-50 shadow-2xl overflow-hidden flex flex-col">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-muted/30">
          <h2 className="text-lg font-semibold text-foreground">{title}</h2>
          <button
            onClick={onClose}
            className="p-2 rounded-[8px] hover:bg-secondary text-muted-foreground transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>
        
        <div className="flex-1 overflow-y-auto p-6">
          <div className="space-y-4">
            {Object.entries(data).map(([key, value]) => (
              <div key={key}>
                <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
                  {key}
                </label>
                {typeof value === 'object' ? (
                  <CodeBlock code={JSON.stringify(value, null, 2)} language="json" showLineNumbers={false} maxHeight="max-h-48" />
                ) : (
                  <div className="bg-secondary border border-border rounded-[8px] px-3 py-2">
                    <code className="text-sm font-mono text-foreground">{String(value)}</code>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
