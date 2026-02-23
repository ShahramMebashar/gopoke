export default function FallbackRenderer({ type, data }) {
  return (
    <div className="rich-fallback">
      <div className="fallback-label">Unknown type: {type}</div>
      <pre className="fallback-raw">{JSON.stringify(data, null, 2)}</pre>
    </div>
  );
}
