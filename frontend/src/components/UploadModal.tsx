import { Fragment, useEffect, useState, type JSX } from "react";
import { Dialog, DialogPanel, DialogTitle, Transition, TransitionChild } from "@headlessui/react";

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmit: (fd: FormData) => void;
};

function TagsInput({ value = [], onChange }: { value: string[]; onChange: (v: string[]) => void; }) {
  const [text, setText] = useState("");
  useEffect(() => setText(""), [value]);

  const add = () => {
    const v = text.split(",").map(s => s.trim()).filter(Boolean);
    if (v.length) onChange([...Array.from(new Set([...value, ...v]))]);
    setText("");
  };
  const remove = (t: string) => onChange(value.filter(x => x !== t));

  return (
    <div>
      <div className="flex gap-2 flex-wrap mb-2">
        {value.map(t => (
          <span key={t} className="flex items-center gap-2 bg-gray-100 text-xs px-2 py-1 rounded-full">
            {t}
            <button type="button" onClick={() => remove(t)} className="text-gray-500">×</button>
          </span>
        ))}
      </div>
      <div className="flex gap-2">
        <input
          value={text}
          onChange={e => setText(e.target.value)}
          onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); add(); } }}
          className="flex-1 rounded border px-3 py-2"
          placeholder="Add tag and press Enter"
        />
        <button type="button" onClick={add} className="px-3 py-2 bg-gray-200 rounded">Add</button>
      </div>
    </div>
  );
}

export default function UploadModal({ open, onClose, onSubmit }: Props): JSX.Element {
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [tags, setTags] = useState<string[]>([]);

  useEffect(() => {
    if (!file) { setPreview(null); return; }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    if (!open) { setFile(null); setPreview(null); setTitle(""); setTags([]); }
  }, [open]);

  const handleSubmit = (formData: FormData) => {
    if (!file) return;
    formData.append("image", file);
    formData.append("title", title);
    formData.append("tags", JSON.stringify(tags));
    onSubmit(formData);
  };

  return (
    <Transition show={open} as={Fragment}>
      <Dialog as="div" className="relative z-50" onClose={onClose}>
        <TransitionChild as={Fragment} enter="ease-out duration-200" enterFrom="opacity-0" enterTo="opacity-100" leave="ease-in duration-150" leaveFrom="opacity-100" leaveTo="opacity-0">
          <div className="fixed inset-0 bg-black/40" />
        </TransitionChild>

        <div className="fixed inset-0 flex items-center justify-center p-4">
          <TransitionChild as={Fragment} enter="ease-out duration-200" enterFrom="translate-y-4 opacity-0" enterTo="translate-y-0 opacity-100" leave="ease-in duration-150" leaveFrom="translate-y-0 opacity-100" leaveTo="translate-y-4 opacity-0">
            <DialogPanel className="bg-white rounded-lg w-full max-w-md p-4">
              <DialogTitle className="text-lg font-semibold mb-3">Upload image</DialogTitle>
              <form action={handleSubmit} className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Image</label>
                  <div className="border rounded p-3 flex items-center justify-center">
                    {preview ? (
                      <img src={preview} alt="preview" className="max-h-48 object-contain" />
                    ) : (
                      <input type="file" accept="image/*" onChange={e => setFile(e.target.files?.[0] ?? null)} />
                    )}
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                  <input value={title} onChange={e => setTitle(e.target.value)} className="w-full rounded border px-3 py-2" />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Tags</label>
                  <TagsInput value={tags} onChange={setTags} />
                </div>

                <div className="flex justify-end gap-2">
                  <button type="button" onClick={onClose} className="px-4 py-2 rounded border">Cancel</button>
                  <button
                    type="submit"
                    disabled={!file || !title.trim() || tags.length === 0}
                    className="px-4 py-2 bg-blue-600 text-white rounded 
             disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Upload
                  </button>
                </div>
              </form>
            </DialogPanel>
          </TransitionChild>
        </div>
      </Dialog>
    </Transition>
  );
}
