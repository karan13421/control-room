export default function CouldBeWrong({ reasons }: { reasons: string[] }) {
  if (reasons.length === 0) return null

  return (
    <section className="could-be-wrong">
      <h3>What could make this wrong</h3>
      <ul>
        {reasons.map((reason) => (
          <li key={reason}>{reason}</li>
        ))}
      </ul>
    </section>
  )
}
