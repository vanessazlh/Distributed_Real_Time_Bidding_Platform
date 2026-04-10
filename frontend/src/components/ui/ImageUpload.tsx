import { useCallback, useRef, useState, type DragEvent, type ChangeEvent } from 'react'
import { api } from '@/lib/api'
import { TextInput } from './FormField'
import { Spinner } from './Spinner'

const ACCEPT = 'image/jpeg,image/png,image/webp,image/gif'
const MAX_SIZE = 5 * 1024 * 1024 // 5 MB

interface ImageUploadProps {
  value: string
  onChange: (url: string) => void
  token: string | null
  label?: string
}

export function ImageUpload({ value, onChange, token, label = 'Image' }: ImageUploadProps) {
  const [mode, setMode] = useState<'upload' | 'url'>('upload')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleFile = useCallback(async (file: File) => {
    setError(null)

    if (!ACCEPT.split(',').includes(file.type)) {
      setError('Only JPEG, PNG, WebP, and GIF files are allowed.')
      return
    }
    if (file.size > MAX_SIZE) {
      setError('File exceeds 5 MB limit.')
      return
    }
    if (!token) {
      setError('You must be signed in to upload.')
      return
    }

    setUploading(true)
    try {
      const url = await api.uploads.image(file, token)
      onChange(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }, [token, onChange])

  const onFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
    e.target.value = ''
  }

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    const file = e.dataTransfer.files?.[0]
    if (file) handleFile(file)
  }

  const onDragOver = (e: DragEvent) => { e.preventDefault(); setDragOver(true) }
  const onDragLeave = () => setDragOver(false)

  // Preview with clear button
  if (value) {
    return (
      <div className="space-y-2">
        <div className="relative inline-block">
          <img
            src={value}
            alt={label}
            className="max-h-40 rounded-lg object-contain border border-border"
          />
          <button
            type="button"
            onClick={() => onChange('')}
            className="absolute -top-2 -right-2 w-6 h-6 rounded-full bg-text-primary text-white text-xs flex items-center justify-center hover:bg-critical transition-colors"
          >
            &times;
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Mode toggle */}
      <div className="flex gap-1 bg-surface-alt rounded-lg p-1 w-fit">
        <button
          type="button"
          onClick={() => { setMode('upload'); setError(null) }}
          className={[
            'px-3 py-1 text-sm rounded-md font-medium transition-all',
            mode === 'upload' ? 'bg-brand text-white' : 'text-text-secondary hover:text-text-primary',
          ].join(' ')}
        >
          Upload
        </button>
        <button
          type="button"
          onClick={() => { setMode('url'); setError(null) }}
          className={[
            'px-3 py-1 text-sm rounded-md font-medium transition-all',
            mode === 'url' ? 'bg-brand text-white' : 'text-text-secondary hover:text-text-primary',
          ].join(' ')}
        >
          Paste URL
        </button>
      </div>

      {mode === 'upload' ? (
        <div>
          <input
            ref={inputRef}
            type="file"
            accept={ACCEPT}
            onChange={onFileChange}
            className="hidden"
          />
          <div
            role="button"
            tabIndex={0}
            onClick={() => inputRef.current?.click()}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') inputRef.current?.click() }}
            onDrop={onDrop}
            onDragOver={onDragOver}
            onDragLeave={onDragLeave}
            className={[
              'border-2 border-dashed rounded-xl p-8 text-center cursor-pointer transition-all',
              dragOver
                ? 'border-brand bg-brand/5'
                : 'border-border hover:border-brand/50',
            ].join(' ')}
          >
            {uploading ? (
              <Spinner />
            ) : (
              <div className="space-y-1">
                <p className="text-text-secondary text-sm">
                  Click to upload or drag and drop
                </p>
                <p className="text-text-secondary/60 text-xs">
                  JPEG, PNG, WebP, or GIF (max 5 MB)
                </p>
              </div>
            )}
          </div>
        </div>
      ) : (
        <TextInput
          type="url"
          placeholder="https://example.com/image.jpg"
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )}

      {error && <p className="text-xs text-critical">{error}</p>}
    </div>
  )
}
