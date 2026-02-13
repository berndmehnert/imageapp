import type { JSX } from "react";
import type { ImageItem } from "../types";

type Props = { items: ImageItem[] };

export default function Feed({ items }: Props): JSX.Element {
  if (items.length === 0) {
    return (
      <div className="text-center text-gray-500 py-12">
        No images found
      </div>
    );
  }

  return (
    <div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
      {items.map(it => (
        <article
          key={it.id}
          className="bg-white rounded-lg shadow-sm overflow-hidden"
        >
          <img
            src={it.thumbnail_url}
            alt={it.title}
            className="w-full h-48 object-cover"
            loading="lazy"
          />
          <div className="p-3">
            <h3 className="font-medium text-sm truncate">{it.title}</h3>
            <div className="mt-2 flex flex-wrap gap-2">
              {(it.tags || []).map(t => (
                <span
                  key={t}
                  className="text-xs bg-gray-100 px-2 py-1 rounded-full"
                >
                  {t}
                </span>
              ))}
            </div>
            {it.score !== undefined && (
              <div className="mt-2 text-xs text-gray-400">
                similarity: {(it.score * 100).toFixed(0)}%
              </div>
            )}
          </div>
        </article>
      ))}
    </div>
  );
}