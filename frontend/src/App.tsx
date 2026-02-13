import { useEffect, useRef, useState, type JSX } from "react";
import type { ImageItem } from "./types";
import Feed from "./components/Feed";
import UploadModal from "./components/UploadModal";

export default function App(): JSX.Element {
  const [items, setItems] = useState<ImageItem[]>([]);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [cursor, setCursor] = useState("");
  const [newCount, setNewCount] = useState(0);
  const [isAtTop, setIsAtTop] = useState(true);

  const wsRef = useRef<WebSocket | null>(null);
  const filterRef = useRef(filter);
  filterRef.current = filter;
  const loadMoreRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!cursor) return;

    const observer = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting) {
        fetchFeed(filter, cursor);
      }
    });

    if (loadMoreRef.current) {
      observer.observe(loadMoreRef.current);
    }

    return () => observer.disconnect();
  }, [cursor, filter]);

  // Track scroll position
  useEffect(() => {
    const handleScroll = () => {
      setIsAtTop(window.scrollY < 100);
    };
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  const fetchFeed = (filterQuery = "", cursorQuery = "") => {
    const params = new URLSearchParams();
    params.set("limit", "20");
    if (filterQuery) params.set("filter", filterQuery);
    if (cursorQuery) params.set("cursor", cursorQuery);

    fetch(`/api/feed?${params}`)
      .then(r => r.json())
      .then(data => {
        if (cursorQuery) {
          setItems(prev => [...prev, ...(data.items || [])]);
        } else {
          setItems(data.items || []);
        }
        setCursor(data.next_cursor || "");
      })
      .catch(() => { });
  };

  useEffect(() => {
    fetchFeed();

    const wsUrl = (location.protocol === "https:" ? "wss://" : "ws://")
      + location.host + "/ws";
    wsRef.current = new WebSocket(wsUrl);
    wsRef.current.onmessage = () => {
      if (!filterRef.current && isAtTop) {
        // At top, no filter → live update!
        fetchFeed();
      } else {
        // Scrolled down or filtering → just notify
        setNewCount(prev => prev + 1);
      }
    };
    return () => wsRef.current?.close();
  }, [isAtTop]);

  useEffect(() => {
    const debounce = setTimeout(() => {
      setNewCount(0);
      fetchFeed(filter);
    }, 300);
    return () => clearTimeout(debounce);
  }, [filter]);

  const loadNew = () => {
    setNewCount(0);
    fetchFeed(filter);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const loadMore = () => {
    if (cursor) fetchFeed(filter, cursor);
  };

  const handleUpload = async (formData: FormData) => {
    await fetch("/api/upload", { method: "POST", body: formData });
    setOpen(false);
  };

  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      {/* Fixed header */}
      <header className="sticky top-0 z-10 bg-gray-50">
        <div className="max-w-4xl mx-auto p-4 flex items-center justify-between">
          <h1 className="text-xl font-semibold">Image Feed</h1>
          <button
            onClick={() => setOpen(true)}
            className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700"
          >
            Upload
          </button>
        </div>

        {/* Filter input */}
        <div className="max-w-4xl mx-auto px-4 pb-4">
          <input
            type="text"
            placeholder="Filter by tags... (e.g. cat, sunset)"
            value={filter}
            onChange={e => setFilter(e.target.value)}
            className="w-full p-3 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          {newCount > 0 && (
            <button
              onClick={loadNew}
              className="w-full my-4 p-2 bg-blue-100 text-blue-700 rounded-lg hover:bg-blue-200"
            >
              {newCount} new {newCount === 1 ? "image" : "images"} available — tap to refresh
            </button>
          )}
        </div>
      </header>

      {/* Scrollable content */}
      <main className="max-w-4xl mx-auto p-4">

        <Feed items={items} />

        {cursor && (
          <div ref={loadMoreRef} className="py-4 text-center text-gray-400 text-sm">
            Loading more...
          </div>
        )}
      </main>

      <UploadModal open={open} onClose={() => setOpen(false)} onSubmit={handleUpload} />
    </div>
  );
}