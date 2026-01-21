import React from 'react';

interface ProgressStepperProps {
  currentStep: number;
  totalSteps: number;
}

export function ProgressStepper({ currentStep, totalSteps }: ProgressStepperProps) {
  return (
    <div className="flex items-center gap-2">
      {Array.from({ length: totalSteps }, (_, i) => (
        <div
          key={i}
          className={`h-1.5 rounded-full transition-all ${
            i < currentStep ? 'w-8 bg-[#E7EDF5]' : 'w-8 bg-[#242C36]'
          }`}
        />
      ))}
    </div>
  );
}
