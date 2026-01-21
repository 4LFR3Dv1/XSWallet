import { useRef, useEffect, useState } from 'react';

interface ChannelGraphProps {
    numChannels: number;
    totalCapacityBTC: number;
}

interface Node {
    id: string;
    x: number;
    y: number;
    type: 'you' | 'peer';
    capacity?: string;
    alias?: string;
}

export function ChannelGraph({ numChannels, totalCapacityBTC }: ChannelGraphProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [hoveredNode, setHoveredNode] = useState<Node | null>(null);

    // Generate nodes state derived from props
    const [nodes, setNodes] = useState<Node[]>([]);

    useEffect(() => {
        if (!containerRef.current) return;

        // Get actual dimensions
        const width = containerRef.current.clientWidth;
        const height = containerRef.current.clientHeight; // Use dynamic height

        const centerX = width / 2;
        const centerY = height / 2;
        // Dynamic radius based on smaller dimension to avoid clipping
        const radius = Math.min(width, height) * 0.35;

        const newNodes: Node[] = [
            { id: 'you', x: centerX, y: centerY, type: 'you', capacity: `${totalCapacityBTC.toFixed(3)} BTC`, alias: 'Your Node' }
        ];

        for (let i = 0; i < numChannels; i++) {
            // Distribute peers in a circle
            const angle = (i / numChannels) * 2 * Math.PI - (Math.PI / 2); // Start from top
            newNodes.push({
                id: `peer-${i}`,
                x: centerX + radius * Math.cos(angle),
                y: centerY + radius * Math.sin(angle),
                type: 'peer',
                capacity: `${(Math.random() * 0.5).toFixed(3)} BTC`,
                alias: `Peer ${i + 1}`
            });
        }
        setNodes(newNodes);
    }, [numChannels, totalCapacityBTC]);

    return (
        <div className="relative w-full h-full" ref={containerRef}>
            <svg className="w-full h-full">
                <defs>
                    <radialGradient id="grad-you" cx="50%" cy="50%" r="50%">
                        <stop offset="0%" stopColor="#7C3AED" stopOpacity="0.8" />
                        <stop offset="100%" stopColor="#7C3AED" stopOpacity="0" />
                    </radialGradient>
                    <filter id="glow">
                        <feGaussianBlur stdDeviation="2.5" result="coloredBlur" />
                        <feMerge>
                            <feMergeNode in="coloredBlur" />
                            <feMergeNode in="SourceGraphic" />
                        </feMerge>
                    </filter>
                </defs>

                {/* Connections */}
                {nodes.slice(1).map((node, i) => (
                    <g key={`conn-${i}`}>
                        {/* Channel Line */}
                        <line
                            x1={nodes[0].x}
                            y1={nodes[0].y}
                            x2={node.x}
                            y2={node.y}
                            stroke="#333"
                            strokeWidth="2"
                        />
                        {/* Animated Flow Pulse */}
                        <circle r="2" fill="#7C3AED">
                            <animateMotion
                                dur={`${2 + i}s`}
                                repeatCount="indefinite"
                                path={`M${nodes[0].x},${nodes[0].y} L${node.x},${node.y}`}
                            />
                        </circle>
                    </g>
                ))}

                {/* Nodes */}
                {nodes.map((node) => (
                    <g
                        key={node.id}
                        onMouseEnter={() => setHoveredNode(node)}
                        onMouseLeave={() => setHoveredNode(null)}
                        style={{ cursor: 'pointer' }}
                    >
                        {node.type === 'you' ? (
                            <>
                                <circle cx={node.x} cy={node.y} r="30" fill="url(#grad-you)" />
                                <circle cx={node.x} cy={node.y} r="12" fill="#7C3AED" stroke="#fff" strokeWidth="2" filter="url(#glow)" />
                                <text x={node.x} y={node.y + 25} textAnchor="middle" fill="white" fontSize="10" fontWeight="bold">YOU</text>
                            </>
                        ) : (
                            <>
                                <circle cx={node.x} cy={node.y} r="8" fill="#1A1A1A" stroke={hoveredNode?.id === node.id ? "#fff" : "#444"} strokeWidth="2" />
                            </>
                        )}
                    </g>
                ))}
            </svg>

            {/* Hover Tooltip */}
            {hoveredNode && (
                <div
                    className="absolute pointer-events-none bg-[#111]/90 backdrop-blur border border-[#333] p-2 rounded-lg text-xs"
                    style={{
                        left: hoveredNode.x + 15,
                        top: hoveredNode.y - 15,
                        zIndex: 10
                    }}
                >
                    <p className="font-bold text-white mb-0.5">{hoveredNode.alias}</p>
                    <p className="text-[#999]">Cap: <span className="text-[#10B981]">{hoveredNode.capacity}</span></p>
                    <p className="text-[#999]">Status: <span className="text-[#10B981]">Active</span></p>
                </div>
            )}
        </div>
    );
}
