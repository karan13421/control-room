import { formatPercent, type Recommendation } from './api'

export default function RecommendationCard({ data }: { data: Recommendation }) {
  const { action, headline, area, window: timeWindow, expected } = data

  if (action !== 'move_drivers') {
    return (
      <section className="card recommendation-card">
        <p className="eyebrow">Hold</p>
        <h2 className="area-name">No move recommended</h2>
        <p className="window">
          {timeWindow.from} to {timeWindow.to}
        </p>
        <p className="headline">{headline}</p>
      </section>
    )
  }

  return (
    <section className="card recommendation-card">
      <p className="eyebrow">Move drivers to</p>
      <h2 className="area-name">{area?.name}</h2>
      <p className="window">
        {timeWindow.from} to {timeWindow.to}
      </p>
      <p className="headline">{headline}</p>
      <div className="stats">
        <div className="stat">
          <span className="stat-label">Now</span>
          <span className="stat-value">{expected.now}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Next hour</span>
          <span className="stat-value">{expected.next}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Rise</span>
          <span className="stat-value">
            +{expected.gap} ({formatPercent(expected.ratio)})
          </span>
        </div>
      </div>
    </section>
  )
}
