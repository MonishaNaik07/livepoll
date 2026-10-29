const base = {
  width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
  strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true,
}

export const EyeIcon = () => (
  <svg {...base}><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
)
export const ShareIcon = () => (
  <svg {...base}><circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" />
    <line x1="8.6" y1="13.5" x2="15.4" y2="17.5" /><line x1="15.4" y1="6.5" x2="8.6" y2="10.5" /></svg>
)
export const TrashIcon = () => (
  <svg {...base}><polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
    <path d="M10 11v6M14 11v6" /><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
)

export const LogoMark = () => (
  <svg width="28" height="28" viewBox="0 0 32 32" aria-hidden="true">
    <rect width="32" height="32" rx="8" fill="var(--brand)" />
    <rect x="7" y="17" width="4" height="8" rx="1" fill="#fff" />
    <rect x="14" y="10" width="4" height="15" rx="1" fill="#fff" />
    <rect x="21" y="14" width="4" height="11" rx="1" fill="#fff" />
  </svg>
)