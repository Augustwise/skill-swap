import type { IconProps } from "./types";

export function PeopleIcon({ size = 28, ...props }: IconProps) {
  return (
    <svg
      {...props}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2.15}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="8.25" cy="8.25" r="3.25" />
      <circle cx="16.5" cy="9.5" r="2.5" />
      <path d="M2.75 20.25c.35-3.55 2.4-5.75 5.5-5.75s5.15 2.2 5.5 5.75" />
      <path d="M15.25 14.85c.4-.12.82-.18 1.25-.18 2.65 0 4.28 1.8 4.75 4.83" />
    </svg>
  );
}
