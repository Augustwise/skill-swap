import type { IconProps } from "./types";

export function ProfileIcon({ size = 28, ...props }: IconProps) {
  return (
    <svg
      {...props}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5.5 19c.6-3.2 3-5 6.5-5s5.9 1.8 6.5 5" />
    </svg>
  );
}
