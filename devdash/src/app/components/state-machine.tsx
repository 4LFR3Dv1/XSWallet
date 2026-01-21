import { Check, Circle, X } from 'lucide-react';

type StateStatus = 'completed' | 'active' | 'pending' | 'failed';

interface State {
  id: string;
  label: string;
  status: StateStatus;
}

interface StateMachineProps {
  states: State[];
  onStateClick?: (stateId: string) => void;
}

export function StateMachine({ states, onStateClick }: StateMachineProps) {
  const getStatusIcon = (status: StateStatus) => {
    switch (status) {
      case 'completed':
        return <Check className="w-4 h-4" />;
      case 'active':
        return <Circle className="w-4 h-4 fill-current" />;
      case 'failed':
        return <X className="w-4 h-4" />;
      default:
        return <Circle className="w-4 h-4" />;
    }
  };

  const getStatusColor = (status: StateStatus) => {
    switch (status) {
      case 'completed':
        return 'bg-success/10 text-success border-success/20';
      case 'active':
        return 'bg-primary/10 text-primary border-primary/20';
      case 'failed':
        return 'bg-error/10 text-error border-error/20';
      default:
        return 'bg-muted text-muted-foreground border-border';
    }
  };

  return (
    <div className="flex items-center gap-2">
      {states.map((state, index) => (
        <div key={state.id} className="flex items-center gap-2">
          <button
            onClick={() => onStateClick?.(state.id)}
            className={`flex items-center gap-2 px-3 py-2 rounded-[8px] border transition-colors ${
              getStatusColor(state.status)
            } ${onStateClick ? 'cursor-pointer hover:opacity-80' : 'cursor-default'}`}
            disabled={!onStateClick}
          >
            {getStatusIcon(state.status)}
            <span className="text-xs font-medium">{state.label}</span>
          </button>
          {index < states.length - 1 && (
            <div className="w-6 h-0.5 bg-border" />
          )}
        </div>
      ))}
    </div>
  );
}
