export const Skeleton = ({ lines = 3 }: { lines?: number }) => (
  <div className="skeleton" aria-busy="true" aria-label="Loading">
    {Array.from({ length: lines }, (_, i) => <div key={i} className="sk-line" style={{ width: `${90 - i * 15}%` }} />)}
  </div>
)
