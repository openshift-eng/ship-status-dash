import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

import type { Tag } from '../types'
import { getTagsEndpoint } from '../utils/endpoints'
import { slugify } from '../utils/slugify'

interface TagsContextValue {
  tags: Tag[]
  loading: boolean
  getTag: (name: string) => Tag | undefined
}

const TagsContext = createContext<TagsContextValue | undefined>(undefined)

export const TagsProvider = ({ children }: { children: ReactNode }) => {
  const [tags, setTags] = useState<Tag[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    fetch(getTagsEndpoint())
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load tags')
        return res.json()
      })
      .then((data) => {
        if (!cancelled) setTags(data ?? [])
      })
      .catch(() => {
        if (!cancelled) setTags([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const getTag = (name: string) => tags.find((t) => slugify(t.name) === slugify(name))

  return <TagsContext.Provider value={{ tags, loading, getTag }}>{children}</TagsContext.Provider>
}

export const useTags = () => {
  const context = useContext(TagsContext)
  if (!context) {
    throw new Error('useTags must be used within a TagsProvider')
  }
  return context
}
