import { useCallback, useEffect, useState } from 'react'
import { fetchRecommendation, type Recommendation } from './api'
import RecommendationCard from './RecommendationCard'
import EvidenceSection from './EvidenceSection'
import CouldBeWrong from './CouldBeWrong'
import './App.css'

function App() {
  // Empty means "live now"; a filled value pins the recommendation to that
  // Chicago wall-clock time instead (past dates get no weather, since the
  // weather API only forecasts a couple of days ahead - the backend already
  // reports that as weather.available: false rather than failing).
  const [at, setAt] = useState('')
  const [data, setData] = useState<Recommendation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback((overrideAt: string) => {
    setLoading(true)
    setError(null)
    fetchRecommendation(overrideAt || undefined)
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load('')
  }, [load])

  return (
    <main className="page">
      <header className="page-header">
        <h1>Operations Control Room</h1>
        <div className="controls">
          <label className="at-control">
            <span>Chicago time</span>
            <input
              type="datetime-local"
              value={at}
              onChange={(e) => {
                setAt(e.target.value)
                load(e.target.value)
              }}
            />
          </label>
          {at && (
            <button
              type="button"
              onClick={() => {
                setAt('')
                load('')
              }}
            >
              Now
            </button>
          )}
          <button type="button" onClick={() => load(at)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

      {error && <p className="error">Failed to load recommendation: {error}</p>}
      {loading && !data && <p className="muted">Loading recommendation…</p>}

      {data && (
        <>
          <RecommendationCard data={data} />
          <EvidenceSection evidence={data.evidence} />
          <CouldBeWrong reasons={data.could_be_wrong} />
        </>
      )}
    </main>
  )
}

export default App
