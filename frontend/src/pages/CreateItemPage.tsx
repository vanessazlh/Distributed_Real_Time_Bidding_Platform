import { useState, useRef, useEffect } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { CATEGORIES } from '@/types'
import { Card, Button, FormField, TextInput, TextArea, StatusBanner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

export default function CreateItemPage() {
  const { shopId }      = useParams<{ shopId: string }>()
  const { user, token, isSeller } = useAuth()
  const navigate        = useNavigate()

  const [title,       setTitle]       = useState('')
  const [description, setDescription] = useState('')
  const [retailValue, setRetailValue] = useState('')
  const [imageUrl,    setImageUrl]    = useState('')
  const [category,    setCategory]    = useState('')
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)
  const [catOpen,     setCatOpen]     = useState(false)
  const catRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (catRef.current && !catRef.current.contains(e.target as Node)) setCatOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to add items"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await api.shops.createItem(
        shopId!,
        {
          title,
          description,
          retail_value: Math.round(parseFloat(retailValue) * 100),
          image_url:    imageUrl  || undefined,
          category:     category  || undefined,
        },
        token!,
      )
      navigate(`/seller/shops/${shopId}/auctions`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer narrow>
      <Link
        to={`/seller/shops/${shopId}/auctions`}
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Shop
      </Link>

      <Card padding="p-8">
        <h1 className="font-display text-3xl text-text-primary mb-2">Add Item</h1>
        <p className="text-text-secondary text-base mb-8">
          List a product that can be auctioned when you have surplus stock.
        </p>

        {error && (
          <div className="mb-4">
            <StatusBanner type="error" message={error} />
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <FormField label="Item Title">
            <TextInput
              type="text"
              required
              placeholder="Mystery Pastry Box (3 items)"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </FormField>

          <FormField label="Description">
            <TextArea
              rows={3}
              placeholder="Describe the item — contents, freshness, best-before, etc."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </FormField>

          <FormField label="Retail Value ($)">
            <TextInput
              type="number"
              required
              min="0.01"
              step="0.01"
              placeholder="28.00"
              value={retailValue}
              onChange={(e) => setRetailValue(e.target.value)}
            />
          </FormField>

          <FormField label="Category">
            <div ref={catRef} className="relative">
              <button
                type="button"
                onClick={() => setCatOpen((o) => !o)}
                className={`w-full flex items-center justify-between px-4 py-3 rounded-xl border-2 font-sans text-base transition-all bg-white ${catOpen ? 'border-brand ring-2 ring-brand/20' : 'border-border hover:border-brand/50'} ${category ? 'text-text-primary' : 'text-text-secondary'}`}
              >
                <span>{category || '— select a category —'}</span>
                <svg className={`w-4 h-4 text-text-secondary transition-transform ${catOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24"><polyline points="6 9 12 15 18 9"/></svg>
              </button>
              {catOpen && (
                <ul className="absolute z-50 mt-1 w-full bg-white border border-border rounded-xl shadow-lg overflow-hidden">
                  {CATEGORIES.map((c) => (
                    <li
                      key={c}
                      onMouseDown={() => { setCategory(c); setCatOpen(false) }}
                      className={`px-4 py-2.5 text-base cursor-pointer transition-colors ${category === c ? 'bg-brand/10 text-brand font-medium' : 'text-text-primary hover:bg-brand/5'}`}
                    >
                      {c}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </FormField>

          <FormField label="Image URL (optional)">
            <TextInput
              type="url"
              placeholder="https://example.com/item.png"
              value={imageUrl}
              onChange={(e) => setImageUrl(e.target.value)}
            />
          </FormField>

          <Button variant="primary" size="lg" type="submit" fullWidth disabled={loading} className="mt-2">
            {loading ? 'Saving…' : 'Add Item'}
          </Button>
        </form>
      </Card>
    </PageContainer>
  )
}
