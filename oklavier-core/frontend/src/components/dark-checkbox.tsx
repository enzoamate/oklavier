"use client";

import { Check, Minus } from "lucide-react";

interface DarkCheckboxProps {
  checked: boolean;
  indeterminate?: boolean;
  onChange: () => void;
  title?: string;
}

export function DarkCheckbox({ checked, indeterminate, onChange, title }: DarkCheckboxProps) {
  return (
    <button
      onClick={(e) => { e.stopPropagation(); onChange(); }}
      title={title}
      className={`size-4 rounded flex items-center justify-center border transition-all cursor-pointer ${
        checked || indeterminate
          ? "bg-oklavier-blue border-oklavier-blue"
          : "bg-white/5 border-white/20 hover:border-white/40"
      }`}
    >
      {checked && <Check className="size-3 text-white" strokeWidth={3} />}
      {!checked && indeterminate && <Minus className="size-3 text-white" strokeWidth={3} />}
    </button>
  );
}
