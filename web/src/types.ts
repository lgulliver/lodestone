export type MetricTone = 'positive' | 'neutral'

export type Metric = {
  label: string
  value: string
  delta?: string
  tone?: MetricTone
  suffix?: string
  caption?: string
}

export type ArtifactStatusTone = 'success' | 'warning' | 'danger'

export type ArtifactRecord = {
  id: string
  name: string
  namespace: string
  type: 'Docker' | 'NPM' | 'Python' | 'Binary' | 'Helm'
  version: string
  pulls: string
  lastUpdated: string
  status: string
  statusTone: ArtifactStatusTone
  isPublic: boolean
  recencyDays: number
}

export type ActivityPoint = {
  day: string
  pulls: number
  publishes: number
}

export type PageLink = number | 'ellipsis'
