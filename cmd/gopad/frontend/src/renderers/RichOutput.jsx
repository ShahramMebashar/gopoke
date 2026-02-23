import { getRenderer } from "./index";

export default function RichOutput({ blocks }) {
  if (!Array.isArray(blocks) || blocks.length === 0) {
    return <div className="rich-empty">No rich output</div>;
  }

  return (
    <div className="rich-output">
      {blocks.map((block, i) => {
        const Renderer = getRenderer(block.type);
        let parsed = block.data;
        if (typeof parsed === "string") {
          try { parsed = JSON.parse(parsed); } catch {}
        }
        return (
          <div key={i} className="rich-block">
            <Renderer type={block.type} data={parsed} />
          </div>
        );
      })}
    </div>
  );
}
