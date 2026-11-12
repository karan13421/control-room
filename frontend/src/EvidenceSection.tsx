import { formatMultiplier, formatPercent, type Evidence } from './api'

export default function EvidenceSection({ evidence }: { evidence: Evidence }) {
  const { weather, past_weeks, weeks_rose, runners_up, data_window } = evidence

  return (
    <section className="card evidence-section">
      <h3>Evidence</h3>

      <p className="evidence-line">
        <strong>Weather: </strong>
        {weather.available
          ? `${weather.condition} (${formatMultiplier(weather.multiplier)})${weather.note ? ` — ${weather.note}` : ''}`
          : 'Weather unavailable, not applied'}
      </p>

      {weeks_rose && <p className="evidence-line">Rose in {weeks_rose} past weeks</p>}

      {past_weeks && past_weeks.length > 0 && (
        <table className="past-weeks">
          <thead>
            <tr>
              <th>Date</th>
              <th>Now</th>
              <th>Next</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {past_weeks.map((w) => (
              <tr key={w.date}>
                <td>{w.date}</td>
                <td>{w.now}</td>
                <td>{w.next}</td>
                <td className={w.rose ? 'rose' : 'fell'}>{w.rose ? '↑ rose' : '↓ fell'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {runners_up.length > 0 && (
        <div className="runners-up">
          <h4>Runners-up</h4>
          <ul>
            {runners_up.map((r) => (
              <li key={r.name}>
                {r.name} — gap {r.gap} ({formatPercent(r.ratio)})
              </li>
            ))}
          </ul>
        </div>
      )}

      <p className="muted data-window">Data window: {data_window}</p>
    </section>
  )
}
