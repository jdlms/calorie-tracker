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
  const viewModes = {
    day: 'day',
    week: 'week',
  }

  let selectedDate = $state(today)
  let selectedView = $state(viewModes.day)
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

  let chartData = $derived(buildChartData())

  let chartOptions = $derived({
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false,
      },
      title: {
        display: false,
      },
      tooltip: {
        backgroundColor: 'rgba(9, 12, 20, 0.95)',
        titleColor: '#f8fafc',
        bodyColor: '#cbd5e1',
        borderColor: 'rgba(148, 163, 184, 0.18)',
        borderWidth: 1,
        padding: 12,
        callbacks: {
          title(items) {
            if (selectedView === viewModes.day) {
              const point = items[0]
              return entries[point.dataIndex]?.description || 'Entry'
            }

            return items[0]?.label || ''
          },
          label(context) {
            if (selectedView === viewModes.day) {
              const entry = entries[context.dataIndex]
              if (!entry) {
                return `${context.parsed.y} kcal`
              }

              return `${formatHourLabel(entry.timestamp)} · ${entry.kcal} kcal`
            }

            const label = context.dataset.label || 'Entry'
            return `${label}: ${context.parsed.y} kcal`
          },
        },
      },
    },
    scales: {
      x: {
        stacked: selectedView === viewModes.week,
        grid: {
          display: false,
        },
        ticks: {
          display: true,
          color: '#94a3b8',
        },
      },
      y: {
        beginAtZero: true,
        stacked: selectedView === viewModes.week,
        ticks: {
          precision: 0,
          color: '#94a3b8',
        },
        grid: {
          color: 'rgba(148, 163, 184, 0.12)',
        },
      },
    },
  })

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

  function buildChartData() {
    if (selectedView === viewModes.day) {
      return buildDayChartData(entries)
    }

    return {
      labels: dailyBreakdown.map((day) => day.label),
      datasets: buildWeekChartDatasets(dailyBreakdown),
    }
  }

  function buildDayChartData(dayEntries) {
    if (dayEntries.length === 0) {
      return {
        labels: [formatShortDate(selectedDate)],
        datasets: [
          {
            label: 'Calories',
            data: [0],
            backgroundColor: 'rgba(51, 65, 85, 0.7)',
            borderRadius: 999,
            borderSkipped: false,
            borderWidth: 0,
            barThickness: 38,
            maxBarThickness: 48,
            categoryPercentage: 0.55,
            barPercentage: 0.7,
          },
        ],
      }
    }

    return {
      labels: dayEntries.map((entry) => formatHourLabel(entry.timestamp)),
      datasets: [
        {
          label: 'Calories',
          data: dayEntries.map((entry) => entry.kcal),
          backgroundColor: dayEntries.map((_, entryIndex) => entryColor(0, entryIndex)),
          borderRadius: 999,
          borderSkipped: false,
          borderWidth: 0,
          barThickness: 34,
          maxBarThickness: 42,
          categoryPercentage: 0.6,
          barPercentage: 0.72,
        },
      ],
    }
  }

  function buildWeekChartDatasets(days) {
    const datasets = []

    days.forEach((day, dayIndex) => {
      day.entries.forEach((entry, entryIndex) => {
        const data = Array(days.length).fill(0)
        data[dayIndex] = entry.kcal
        const isTopEntry = entryIndex === day.entries.length - 1

        datasets.push({
          label: entry.description || `Entry ${entryIndex + 1}`,
          data,
          backgroundColor: entryColor(dayIndex, entryIndex),
          borderColor: 'rgba(15, 23, 42, 0.78)',
          borderWidth: 1,
          borderRadius: isTopEntry
            ? { topLeft: 18, topRight: 18, bottomLeft: 0, bottomRight: 0 }
            : 0,
          borderSkipped: false,
          stack: 'calories',
          barThickness: 28,
          maxBarThickness: 36,
          categoryPercentage: 0.7,
          barPercentage: 0.96,
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
        backgroundColor: 'rgba(51, 65, 85, 0.7)',
        borderRadius: { topLeft: 18, topRight: 18, bottomLeft: 0, bottomRight: 0 },
        borderSkipped: false,
        borderWidth: 0,
        stack: 'calories',
        barThickness: 28,
        maxBarThickness: 36,
        categoryPercentage: 0.7,
        barPercentage: 0.96,
      },
    ]
  }

  function entryColor(dayIndex, entryIndex) {
    const hue = (dayIndex * 41 + entryIndex * 31 + 28) % 360
    return `hsl(${hue} 72% 62%)`
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

  function formatHourLabel(timestamp) {
    return new Date(timestamp * 1000).toLocaleTimeString([], {
      hour: 'numeric',
    })
  }
</script>

<main class="shell">
  <section class="hero">
    <div class="logo" aria-hidden="true">🍞</div>
  </section>

  <section class="card totals-card">
    <div class="totals-grid">
      <div>
        <span>Calories</span>
        <strong>{totals.kcal}</strong>
      </div>
      <div>
        <span>Protein</span>
        <strong>{totals.protein}g</strong>
      </div>
      <div>
        <span>Fat</span>
        <strong>{totals.fat}g</strong>
      </div>
      <div>
        <span>Carbs</span>
        <strong>{totals.carbs}g</strong>
      </div>
    </div>
  </section>

  <section class="card chart-card">
    <div class="chart-header">
      <div>
        <h2>{selectedView === viewModes.day ? 'Day view' : 'Week view'}</h2>
        <p>
          {selectedView === viewModes.day
            ? 'Each bar segment is one entry from the day.'
            : 'Each day shows stacked entries across the last 7 days.'}
        </p>
      </div>
    </div>

    {#if chartLoading && selectedView === viewModes.week}
      <p class="status">Loading chart...</p>
    {:else if loading && selectedView === viewModes.day}
      <p class="status">Loading chart...</p>
    {:else if chartError}
      <p class="status error">{chartError}</p>
    {:else if error && selectedView === viewModes.day}
      <p class="status error">{error}</p>
    {:else}
      <div class:selected-day-height={selectedView === viewModes.day} class:week-height={selectedView === viewModes.week} class="chart-wrap">
        <Bar data={chartData} options={chartOptions} />
      </div>
    {/if}

    <div class="controls">
      <p class="selected-date">{formatShortDate(selectedDate)}</p>

      <div class="button-row">
        <button type="button" class="ghost" onclick={() => shiftDate(-1)}>← Prev</button>
        <button type="button" class:selected={selectedView === viewModes.day} onclick={() => (selectedView = viewModes.day)}>Day</button>
        <button type="button" class:selected={selectedView === viewModes.week} onclick={() => (selectedView = viewModes.week)}>Week</button>
        <button type="button" class="ghost" onclick={() => shiftDate(1)}>Next →</button>
      </div>

      <div class="button-row">
        <input type="date" bind:value={selectedDate} />
        <button type="button" class="ghost" onclick={() => (selectedDate = today)}>Today</button>
      </div>
    </div>
  </section>
</main>

<style>
  :global(html) {
    background: #020617;
  }

  :global(body) {
    margin: 0;
    min-height: 100vh;
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    background:
      radial-gradient(circle at top, rgba(71, 85, 105, 0.18), transparent 35%),
      linear-gradient(180deg, #020617 0%, #0f172a 100%);
    background-color: #020617;
    color: #f8fafc;
  }

  :global(*) {
    box-sizing: border-box;
  }

  .shell {
    width: min(920px, calc(100vw - 2rem));
    margin: 0 auto;
    padding: 2rem 0 3rem;
  }

  .hero {
    text-align: center;
    margin-bottom: 3rem;
  }

  .logo {
    font-size: 3rem;
    line-height: 1;
    margin-bottom: 0.75rem;
  }

  .chart-header p,
  .status {
    color: #94a3b8;
  }

  .card {
    background: rgba(15, 23, 42, 0.82);
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 24px;
    box-shadow: 0 30px 80px rgba(2, 6, 23, 0.45);
    backdrop-filter: blur(14px);
  }

  .totals-card {
    margin-bottom: 1.25rem;
    padding: 1.25rem;
  }

  .totals-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.85rem;
    text-align: center;
  }

  .totals-grid div {
    padding: 1rem;
    border-radius: 18px;
    background: rgba(30, 41, 59, 0.72);
  }

  .totals-grid span {
    display: block;
    margin-bottom: 0.4rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: #94a3b8;
  }

  .totals-grid strong {
    font-size: 1.6rem;
    letter-spacing: -0.03em;
  }

  .chart-card {
    padding: 1.25rem;
  }

  .chart-header h2 {
    margin: 0 0 0.25rem;
    font-size: 1.2rem;
  }

  .chart-header p {
    margin: 0 0 1rem;
  }

  .chart-wrap {
    margin-bottom: 1rem;
  }

  .selected-day-height {
    height: 380px;
  }

  .week-height {
    height: 320px;
  }

  .controls {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    align-items: center;
  }

  .selected-date {
    margin: 0;
    font-size: 0.95rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #cbd5e1;
  }

  .button-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    justify-content: center;
  }

  button,
  input[type='date'] {
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 999px;
    background: rgba(30, 41, 59, 0.9);
    color: #f8fafc;
    padding: 0.8rem 1rem;
    font: inherit;
  }

  button {
    cursor: pointer;
    transition: transform 0.16s ease, background 0.16s ease, border-color 0.16s ease;
  }

  button:hover {
    transform: translateY(-1px);
    background: rgba(51, 65, 85, 0.95);
  }

  button.selected {
    background: linear-gradient(135deg, #f97316, #fb7185);
    border-color: transparent;
    color: white;
  }

  button.ghost {
    background: rgba(15, 23, 42, 0.95);
  }

  .error {
    color: #fda4af;
  }

  @media (max-width: 720px) {
    .shell {
      width: min(100vw - 1rem, 920px);
      padding-top: 1rem;
    }

    .totals-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .selected-day-height {
      height: 300px;
    }

    .week-height {
      height: 280px;
    }
  }
</style>
