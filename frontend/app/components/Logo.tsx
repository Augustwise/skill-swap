export function Logo() {
  return (
    <a className="logo" href="#top" aria-label="SkillSwap — на початок сторінки">
      <svg viewBox="0 0 32 32" aria-hidden="true">
        <defs>
          <linearGradient id="logo-gradient" x1="3" y1="28" x2="28" y2="3">
            <stop stopColor="#7c69ff" />
            <stop offset="1" stopColor="#4827f5" />
          </linearGradient>
        </defs>
        <circle cx="16" cy="16" r="14" fill="url(#logo-gradient)" />
        <path
          d="M4.5 18.5c5.4 1.9 9.8.1 13.3-5.3 1.8-2.8 4.5-3.3 9-1.9-2.2 1-3.7 2.6-4.7 4.8-2.7 5.8-8.4 8-17.6 2.4Z"
          fill="white"
          fillOpacity=".88"
        />
        <circle cx="22.8" cy="8.5" r="2.2" fill="#a69aff" />
      </svg>
      <span>SkillSwap</span>
    </a>
  );
}
