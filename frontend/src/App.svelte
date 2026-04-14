<svelte:head>
  <title>Calorie Tracker</title>
</svelte:head>

<script>
  import { onMount } from 'svelte'
  import { Bar } from 'svelte-chartjs'
  import {
    BarElement,
    CategoryScale,
    Chart as ChartJS,
    Legend,
    LinearScale,
    Title,
    Tooltip,
  } from 'chart.js'

  ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

  const today = new Date().toISOString().slice(0, 10)

  let selectedDate = $state(today)
  let entries = $state([])
  let loading = $state(false)
  let error = $state('')
  let chartLoading = $state(false)
  let chartError = $state('')
  let dailyBreakdown = $state([])

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

  let chartData = $derived({
    labels: dailyBreakdown.map((day) => day.label),
    datasets: buildChartDatasets(dailyBreakdown),
  })

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false,
      },
      title: {
        display: true,
        text: 'Last 7 days',
      },
    },
    scales: {
      x: {
        stacked: true,
      },
      y: {
        beginAtZero: true,
        stacked: true,
        ticks: {
          precision: 0,
        },
      },
    },
  }

  let lastLoadedDate = ''
  let lastChartAnchorDate = ''

  onMount(() => {
    loadEntries(selectedDate)
    loadChartData(selectedDate)
  })

  $effect(() => {
    if (!selectedDate || selectedDate === lastLoadedDate) {
      return
    }

    loadEntries(selectedDate)
  })

  $effect(() => {
    if (!selectedDate || selectedDate === lastChartAnchorDate) {
      return
    }

    loadChartData(selectedDate)
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

  async function loadChartData(anchorDate) {
    lastChartAnchorDate = anchorDate
    chartLoading = true
    chartError = ''

    try {
      const dates = buildDateRange(anchorDate, 7)
      const responses = await Promise.all(
        dates.map(async (date) => {
          const response = await fetch(`/api/entries?date=${date}`)
          if (!response.ok) {
            throw new Error(`Request failed with status ${response.status}`)
          }

          const payload = await response.json()
          const dayEntries = payload.entries ?? []

          return {
            date,
            label: formatShortDate(date),
            entries: dayEntries,
          }
        }),
      )

      dailyBreakdown = responses
    } catch (err) {
      dailyBreakdown = []
      chartError = err instanceof Error ? err.message : 'Failed to load chart data'
    } finally {
      chartLoading = false
    }
  }

  function buildChartDatasets(days) {
    const datasets = []

    days.forEach((day, dayIndex) => {
      day.entries.forEach((entry, entryIndex) => {
        const data = Array(days.length).fill(0)
        data[dayIndex] = entry.kcal

        datasets.push({
          label: entry.description || `Entry ${entryIndex + 1}`,
          data,
          backgroundColor: pastelColor(dayIndex, entryIndex),
          borderWidth: 0,
          stack: 'calories',
        })
      })
    })

    if (datasets.length > 0) {
      return datasets
    }

    return [
      {
        label: 'Calories',
        data: days.map(() => 0),
        backgroundColor: '#d9d9d9',
        borderWidth: 0,
        stack: 'calories',
      },
    ]
  }

  function pastelColor(dayIndex, entryIndex) {
    const hue = (dayIndex * 47 + entryIndex * 29) % 360
    return `hsl(${hue} 65% 78%)`
  }

  function buildDateRange(anchorDate, days) {
    const end = new Date(`${anchorDate}T00:00:00Z`)
    const dates = []

    for (let i = days - 1; i >= 0; i -= 1) {
      const current = new Date(end)
      current.setUTCDate(end.getUTCDate() - i)
      dates.push(current.toISOString().slice(0, 10))
    }

    return dates
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

  function formatShortDate(date) {
    return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, {
      month: 'short',
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
    <h2>Calories by day</h2>

    {#if chartLoading}
      <p>Loading chart...</p>
    {:else if chartError}
      <p>{chartError}</p>
    {:else}
      <div style="height: 320px;">
        <Bar data={chartData} options={chartOptions} />
      </div>
    {/if}
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
