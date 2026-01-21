import { useState } from 'react';
import { Copy, Check, WrapText, Hash } from 'lucide-react';

interface CodeBlockProps {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
  title?: string;
}

export function CodeBlock({ code, language = 'text', showLineNumbers = true, title }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const [wrapped, setWrapped] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const lines = code.split('\n');

  // Syntax highlighting for common Bitcoin Script opcodes
  const highlightCode = (line: string) => {
    return line
      .replace(/(OP_\w+)/g, '<span class="text-[#F7931A]">$1</span>')
      .replace(/(<[^>]+>)/g, '<span class="text-[#9945FF]">$1</span>')
      .replace(/(\d+)/g, '<span class="text-[#00B4E6]">$1</span>');
  };

  return (
    <div className="bg-[#0d0d0d] border border-[#252525] rounded-xl overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-[#252525] bg-[#141414]">
        <div className="flex items-center gap-2">
          {title && <span className="text-xs font-medium text-[#666]">{title}</span>}
          <span className="px-2 py-0.5 text-[9px] font-medium text-[#555] bg-[#1a1a1a] rounded uppercase tracking-wider">
            {language}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setWrapped(!wrapped)}
            className={`p-1.5 rounded-md transition-colors ${wrapped ? 'bg-white/10 text-white' : 'text-[#555] hover:text-white hover:bg-white/5'}`}
            title="Toggle word wrap"
          >
            <WrapText className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={handleCopy}
            className="p-1.5 rounded-md text-[#555] hover:text-white hover:bg-white/5 transition-colors"
            title="Copy code"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-[#10B981]" /> : <Copy className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* Code Content */}
      <div className={`p-4 overflow-x-auto ${wrapped ? 'whitespace-pre-wrap' : 'whitespace-pre'}`}>
        <code className="text-sm font-mono">
          {lines.map((line, index) => (
            <div key={index} className="flex">
              {showLineNumbers && (
                <span className="select-none text-[#333] w-8 flex-shrink-0 text-right pr-4">
                  {index + 1}
                </span>
              )}
              <span
                className="text-[#ccc]"
                dangerouslySetInnerHTML={{ __html: highlightCode(line) }}
              />
            </div>
          ))}
        </code>
      </div>
    </div>
  );
}
