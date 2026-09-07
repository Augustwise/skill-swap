import type { IconProps } from "./types";

export function BoltIcon({ size = 28, ...props }: IconProps) {
  return (
    <svg
      {...props}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="currentColor"
      stroke="none"
      aria-hidden="true"
    >
      <path d="M13.3 2.8 5.8 13.1c-.5.7 0 1.7.9 1.7h4.1l-.2 6.4 7.6-10.4c.5-.7 0-1.7-.9-1.7h-4.1l.1-6.3Z" />
    </svg>
  );
}
