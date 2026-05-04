"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";

interface Option {
  value: string;
  label: string;
}

interface DarkSelectProps {
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  className?: string;
  placeholder?: string;
  dropUp?: boolean;
  fullWidth?: boolean;
}

export function DarkSelect({ value, options, onChange, className, placeholder, dropUp, fullWidth }: DarkSelectProps) {
  const [open, setOpen] = useState(false);
  const selected = options.find((o) => o.value === value);

  return (
    <div className={`relative ${className || ""}`}>
      <button
        onClick={() => setOpen(!open)}
        className={`flex items-center justify-between gap-2 border border-white/10 rounded-lg px-3 text-sm text-white/70 hover:text-white transition-colors ${
          fullWidth ? "w-full bg-[#2e3862]/80 py-2 rounded-md" : "bg-[#1a1f36] py-1.5"
        }`}
      >
        {selected?.label || placeholder || value}
        <ChevronDown className={`size-3.5 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className={`absolute left-0 right-0 z-50 min-w-[12rem] bg-[#1a1f36] border border-white/10 rounded-xl shadow-2xl overflow-hidden ${dropUp ? "bottom-10" : "top-full mt-1"}`}>
            {options.map((o) => (
              <button
                key={o.value}
                onClick={() => { onChange(o.value); setOpen(false); }}
                className={`w-full text-left px-4 py-2 text-sm transition-colors ${
                  value === o.value
                    ? "bg-oklavier-blue/20 text-white"
                    : "text-white/60 hover:bg-white/5"
                }`}
              >
                {o.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
