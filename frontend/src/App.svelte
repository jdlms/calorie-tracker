<svelte:head>
  <title>Calorie Tracker</title>
</svelte:head>

<script>
  import { onMount } from 'svelte'

  const today = new Date().toISOString().slice(0, 10)

  let selectedDate = $state(today)
  let entries = $state([])
  let loading = $state(false)
  let error = $state('')

  let totals = $derived(
    entries.reduce(
      (acc, entry) => {
        acc.kcal += entry.kcal
        acc.protein += entry.protein
        acc.fat += entry.fat
        acc.carbs += entry.carbs
        return acc
      },
      { kcal: 0, protein: 0, fat: 0, carbs: 0 },
    ),
  )

  let lastLoadedDate = ''

  onMount(() => {
    loadEntries(selectedDate)
  })

  $effect(() => {
    if (!selectedDate || selectedDate === lastLoadedDate) {
      return
    }

    loadEntries(selectedDate)
  })

  async function loadEntries(date) {
    lastLoadedDate = date
    loading = true
    error = ''

    try {
      const response = await fetch(`/api/entries?date=${date}`)
      if (!response.ok) {
        throw new Error(`Request failed with status ${response.status}`)
      }

      const payload = await response.json()
      entries = payload.entries ?? []
    } catch (err) {
      entries = []
      error = err instanceof Error ? err.message : 'Failed to load entries'
    } finally {
      loading = false
    }
  }

  function shiftDate(days) {
    const date = new Date(`${selectedDate}T00:00:00Z`)
    date.setUTCDate(date.getUTCDate() + days)
    selectedDate = date.toISOString().slice(0, 10)
  }

  function formatDate(date) {
    return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      timeZone: 'UTC',
    })
  }

  function formatTimestamp(timestamp) {
    return new Date(timestamp * 1000).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })
  }
</script>

<main>
  <h1>Calorie Tracker</h1>

  <section>
    <div>
      <button type="button" onclick={() => shiftDate(-1)}>Previous day</button>
      <input type="date" bind:value={selectedDate} />
      <button type="button" onclick={() => (selectedDate = today)}>Today</button>
      <button type="button" onclick={() => shiftDate(1)}>Next day</button>
    </div>

    <p>{formatDate(selectedDate)}</p>
  </section>

  <section>
    <h2>Daily totals</h2>
    <ul>
      <li>Calories: {totals.kcal}</li>
      <li>Protein: {totals.protein}g</li>
      <li>Fat: {totals.fat}g</li>
      <li>Carbs: {totals.carbs}g</li>
    </ul>
  </section>

  <section>
    <h2>Entries</h2>

    {#if loading}
      <p>Loading...</p>
    {:else if error}
      <p>{error}</p>
    {:else if entries.length === 0}
      <p>No entries for this day yet.</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>Time</th>
            <th>Description</th>
            <th>Calories</th>
            <th>Protein</th>
            <th>Fat</th>
            <th>Carbs</th>
          </tr>
        </thead>
        <tbody>
          {#each entries as entry}
            <tr>
              <td>{formatTimestamp(entry.timestamp)}</td>
              <td>{entry.description}</td>
              <td>{entry.kcal}</td>
              <td>{entry.protein}g</td>
              <td>{entry.fat}g</td>
              <td>{entry.carbs}g</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</main>
