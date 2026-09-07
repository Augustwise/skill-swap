import type { IconProps } from "./types";

export function SproutIcon({ size = 28, ...props }: IconProps) {
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
      <path d="M12 21V11M12 15c-5.2 0-7.2-3.2-7.3-7.1 3.8 0 6.4 1.5 7.3 4.4M12 12.8c.9-4.2 3.6-6 7.5-6 .1 4.1-2.2 7.2-7.5 7.2" />
    </svg>
  );
}
