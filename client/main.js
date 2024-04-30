(async () => {

const api = (document.location.host.indexOf('localhost') !== -1 ?
  'http://localhost:10405' :
  document.location.origin)
const apiUrl = new URL(api)
const form = (dict) => {
  const d = new URLSearchParams()
  for (const [k, v] of Object.entries(dict))
    d.set(k, v)
  return d
}

const search = new URL(document.location).searchParams
const uid = +(search.get('uid') || prompt('User ID'))
const rid = +(search.get('room') || prompt('Room ID'))
document.cookie = `auth=!${uid}; SameSite=Lax; Path=/; Max-Age=604800`
window.history.replaceState(null, null, `?room=${rid}&uid=${uid}`)

const user = await (await fetch(`${api}/me`, { credentials: 'include' })).json()
const room = await (await fetch(`${api}/room/${rid}`, { credentials: 'include' })).json()

const cardSet = await (await fetch(`cards.json`)).json()
const cogFn = ['Se', 'Si', 'Ne', 'Ni', 'Te', 'Ti', 'Fe', 'Fi']
const relAspect = ['💥激情', '💚亲密', '⛰️责任']
const resultText = {2: '大成功', 1: '成功', '-1': '失败', '-2': '大失败', null: '无'}

document.getElementById('uid').innerText = uid
document.getElementById('nickname').innerText = user.nickname

document.getElementById('rid').innerText = rid
document.getElementById('room-title').innerText = room.title
document.getElementById('room-tags').innerText = room.tags.join(' · ')
document.getElementById('room-creator').innerText = room.creator.nickname

const elStateMsg = document.getElementById('state-msg')
const error = (msg) => { elStateMsg.innerText = msg; elStateMsg.classList = 'error' }
const info = (msg) => { elStateMsg.innerText = msg; elStateMsg.classList = 'info' }

const tryParse = (text) => {
  try {
    return JSON.parse(text)
  } catch (e) {
    return {}
  }
}

let ws
const reconnect = () => {
  ws = new WebSocket(
    (apiUrl.protocol === 'https:' ? 'wss:' : 'ws:') +
    `//${apiUrl.host}/room/${rid}/channel`
  )
  ws.onopen = () => {
    info('Connected')
  }
  ws.onclose = () => {
    error('Connection lost')
  }
  ws.onmessage = (evt) => {
    const o = JSON.parse(evt.data)
    console.log(o)
    if (o.type === 'log') {
      processLogs(o.log)
    } else if (o.type === 'room_state') {
      processLogs([{
        id: -1,
        timestamp: Math.floor(Date.now() / 1000),
        content: `房间【${o.room.title}】：${o.room.description}`
      }])
      updatePlayers(o.players)
      if (o.phase === 'assembly') {
        updateAssemblyPanel(o.players)
      } else if (o.phase === 'appointment') {
        updateAppointmentPanel(o.appointment_status)
      } else if (o.phase === 'gameplay') {
        updateGameplayPanel(o.gameplay_status)
      }
    } else if (o.type === 'assembly_update') {
      updatePlayers(o.players)
      updateAssemblyPanel(o.players)
    } else if (o.type === 'start') {
      updateAppointmentPanel({ holder: o.holder, timer: 30 })
    } else if (o.type === 'appointment_pass') {
      updateAppointmentPanel({ holder: o.next_holder, timer: 30 })
    } else if (o.type === 'appointment_accept') {
      updateGameplayPanel(o.gameplay_status)
    } else if (o.type === 'gameplay_progress') {
      updateGameplayPanel(o.gameplay_status)
    } else if (o.type === 'game_end') {
      updateGameEndPanel(o)
    }
  }
}

const send = (o) => ws.send(JSON.stringify(o))

const htmlEscape = (s) =>
  (s || '')
   .replaceAll('&', '&amp;')
   .replaceAll('<', '&lt;')
   .replaceAll('>', '&gt;')
   .replaceAll('"', '&quot;')
   .replaceAll("'", '&apos;')

const processLogs = (logs) => {
  const elContainer = document.getElementById('log')
  for (const l of logs) {
    const id = `log-id-${l.id}`
    if (document.getElementById(id)) continue
    const node = document.createElement('p')
    node.id = id
    node.innerHTML = `<span class='timestamp'>${(new Date(l.timestamp * 1000)).toISOString().substring(11, 19)}</span> ${htmlEscape(l.content)}`
    elContainer.appendChild(node)
  }
  elContainer.scrollTo(0, elContainer.scrollHeight)
}

const sendComment = () => {
  if (document.getElementById('txt-comment').value !== '') {
    send({ type: 'comment', text: document.getElementById('txt-comment').value })
    document.getElementById('txt-comment').value = ''
  }
}
document.getElementById('txt-comment').addEventListener('keypress', (e) => {
  if (e.keyCode === 13) sendComment()
})
document.getElementById('btn-comment').addEventListener('click', (e) => sendComment())

let lastSavedPlayers
let myIndex

const updatePlayers = (playerProfiles) => {
  lastSavedPlayers= playerProfiles

  const elContainer = document.getElementById('players')
  elContainer.innerHTML = ''
  for (const [i, pf] of Object.entries(playerProfiles)) {
    const node = document.createElement('p')
    node.id = `player-id-${i}`
    node.innerHTML =
      (pf.creator.id === uid ? '<strong>' : '') +
      (pf.id === null ? '[未入座]' : `[${+i + 1}]`) +
      ` ${htmlEscape(pf.creator.nickname)}` +
      (pf.creator.id === uid ? ' (👋 自己)</strong>' : '') +
      (pf.id === null ? '' : ` — ${htmlEscape(pf.details.race)}; ${formatStats(pf.stats)}`)
    elContainer.appendChild(node)

    const marker = document.createElement('span')
    marker.id = `player-marker-${i}`
    marker.innerText = '⬤'
    marker.classList.add('player-marker')
    node.prepend(marker)

    if (pf.creator.id === uid) myIndex = +i
  }
}

// const formatStats = (stats) => stats.map((n, i) => `${cogFn[i]}=${n}`).join(', ')
const formatStats = (stats) =>
  stats.map((n, i) => `<span class='cog-fn-${cogFn[i]}'>${cogFn[i]} <strong>${n}</strong></span>`).join(' ')
const formatCogFns = (fns) =>
  fns.map((i) => `<span class='cog-fn-${cogFn[i]}'>${cogFn[i]}</span>`).join(' ')
const formatRelationship = (rel, plus) =>
  `<span class='inline-block'>` + rel.map((n, i) => `<span class='rel-aspect-${i}'>${relAspect[i]} <strong>${plus && n > 0 ? '+' : ''}${n}</strong></span>`).join(' ') + `</span>`

const markPlayer = (index, storytellerIndex) => {
  const elContainer = document.getElementById('players')
  for (const node of elContainer.children) {
    const i = +node.id.substring('player-id-'.length)
    const marker = document.getElementById(`player-marker-${i}`)

    if (i === index) marker.classList.add('active')
    else marker.classList.remove('active')

    if (i === storytellerIndex) marker.classList.add('storyteller')
    else marker.classList.remove('storyteller')
  }
}

// Timer
const elTimerContainer = document.getElementById('timer')
const elTimerDisplay = document.getElementById('timer-display')
let timerExpiresAt
let timeoutId

const updateTimerDisplay = () => {
  if (timerExpiresAt === undefined) return
  const remaining = timerExpiresAt - Date.now()
  const remainingSeconds = Math.ceil(remaining / 1000 - 0.1)
  elTimerDisplay.innerText = remainingSeconds
  if (remainingSeconds <= 0) {
    timeoutId = undefined
  } else {
    setTimeout(updateTimerDisplay,
      (timerExpiresAt - (remainingSeconds - 1) * 1000) - Date.now())
  }
}
const timerSet = (seconds) => {
  if (timeoutId !== undefined) clearTimeout(timeoutId)
  timeoutId = undefined
  if (seconds === null) {
    elTimerContainer.classList.add('invisible')
    timerExpiresAt = undefined
  } else {
    elTimerContainer.classList.remove('invisible')
    timerExpiresAt = Date.now() + seconds * 1000
    updateTimerDisplay()
  }
}

////// Assembly panel //////

const profileRepr = (pf, omitId) => `${omitId ? '' : `[${pf.id}]: `}种族 <strong>${htmlEscape(pf.details.race)}</strong>，背景 "<strong>${htmlEscape(pf.details.description)}</strong>", ${formatStats(pf.stats)}`

const showAssemblyPanel = () => {
  document.getElementById('assembly-panel').classList.remove('hidden')
  document.getElementById('appointment-panel').classList.add('hidden')
  document.getElementById('gameplay-panel').classList.add('hidden')
  document.getElementById('game-end-panel').classList.add('hidden')
  document.getElementById('players').classList.add('assembly')
  document.getElementById('progress-indicator').classList.add('hidden')
}
const showSeatAndProfiles = () => {
  document.getElementById('seat-profiles').classList.remove('hidden')
  document.getElementById('seat-withdraw').classList.add('hidden')
}
const showSeatWithdraw = (p) => {
  document.getElementById('seat-profiles').classList.add('hidden')
  document.getElementById('seat-withdraw').classList.remove('hidden')
  document.getElementById('seated-profile').innerHTML = profileRepr(p)
}
const updateAssemblyPanel = (players) => {
  showAssemblyPanel()
  const p = players.find((p) => p.creator.id === uid && p.id !== null)
  if (p)
    showSeatWithdraw(p)
  else
    showSeatAndProfiles()

  if (room.creator.id === uid) {
    document.getElementById('btn-start').disabled =
      players.length <= 1 || players.some((p) => p.id === null);
  }

  timerSet(null)
}
if (room.creator.id === uid) {
  document.getElementById('seat-start').classList.remove('hidden')
  document.getElementById('btn-start').addEventListener('click', (e) => {
    send({ type: 'start' })
  })
}

const addProfileButton = (pf) => {
  const elContainer = document.getElementById('profiles')
  const node = document.createElement('button')
  node.innerHTML = `角色档案 [${pf.id}]`
  node.addEventListener('click', (e) => {
    send({
      type: 'seat',
      profile_id: pf.id,
    })
  })
  elContainer.appendChild(node)

  const nodeDesc = document.createElement('span')
  nodeDesc.innerHTML = ` — ${profileRepr(pf, true)}`
  elContainer.appendChild(nodeDesc)

  elContainer.appendChild(document.createElement('br'))
}
const profiles = await (await fetch(`${api}/profile/my`, { credentials: 'include' })).json()
for (const pf of profiles) addProfileButton(pf)

const updateSum = () => {
  const sum = [1, 2, 3, 4, 5, 6, 7, 8]
    .map((i) => +document.getElementById(`stat-${i}`).value)
    .reduce((a, b) => a + b)
  document.getElementById('profile-stat-sum').innerText = sum
  document.getElementById('btn-new-profile').disabled = (sum > 480)
}
for (let i = 1; i <= 8; i++) {
  const el = document.getElementById(`stat-${i}`)
  const elValue = document.getElementById(`value-stat-${i}`)
  el.addEventListener('input', (e) => {
    elValue.innerText = el.value
    updateSum()
  })
  elValue.innerText = el.value
}
updateSum()

document.getElementById('btn-stats-random').addEventListener('click', (e) => {
  const values = []

  do {
    // Sample 7 unique elements in [0, 407)
    const samples = new Set()
    for (let i = 400; i < 407; i++) {
      let x = Math.floor(Math.random() * (i + 1))
      if (samples.has(x)) x = i
      samples.add(x)
    }
    const sorted = [-1, 407, ...samples.values()].sort((a, b) => a - b)
    for (let i = 0; i < 8; i++)
      values[i] = sorted[i + 1] - sorted[i] - 1 + 10
  } while (!values.every((n) => n <= 80))

  for (let i = 1; i <= 8; i++) {
    document.getElementById(`stat-${i}`).value = values[i - 1]
    document.getElementById(`value-stat-${i}`).innerText = values[i - 1]
  }
  updateSum()
})

document.getElementById('btn-new-profile').addEventListener('click', async (e) => {
  const resp = await (await fetch(`${api}/profile/create`, {
    credentials: 'include',
    method: 'POST',
    body: form({
      details: JSON.stringify({
        race: document.getElementById('pf-race').value,
        description: document.getElementById('pf-background').value,
      }),
      stats: [1, 2, 3, 4, 5, 6, 7, 8]
        .map((i) => document.getElementById(`stat-${i}`).value.toString())
        .join(','),
      traits: '',
    }),
  })).json()
  addProfileButton(resp)
})

document.getElementById('btn-seat-withdraw').addEventListener('click', (e) => {
  send({
    type: 'withdraw',
  })
})

////// Appointment panel //////

const showAppointmentPanel = () => {
  document.getElementById('assembly-panel').classList.add('hidden')
  document.getElementById('appointment-panel').classList.remove('hidden')
  document.getElementById('gameplay-panel').classList.add('hidden')
  document.getElementById('game-end-panel').classList.add('hidden')
  document.getElementById('players').classList.remove('assembly')
  document.getElementById('progress-indicator').classList.add('hidden')
}
const updateAppointmentPanel = (status) => {
  showAppointmentPanel()
  markPlayer(status.holder)
  if (status.holder === myIndex) {
    document.getElementById('appointment-wait').classList.add('hidden')
    document.getElementById('appointment-ask').classList.remove('hidden')
  } else {
    document.getElementById('appointment-wait').classList.remove('hidden')
    document.getElementById('appointment-ask').classList.add('hidden')
  }
  timerSet(status.timer)
}

document.getElementById('btn-appointment-accept').addEventListener('click', (e) => {
  send({ type: 'appointment_accept' })
})
document.getElementById('btn-appointment-pass').addEventListener('click', (e) => {
  send({ type: 'appointment_pass' })
})

////// Gameplay panel //////

const showGameplayPanel = () => {
  document.getElementById('assembly-panel').classList.add('hidden')
  document.getElementById('appointment-panel').classList.add('hidden')
  document.getElementById('gameplay-panel').classList.remove('hidden')
  document.getElementById('game-end-panel').classList.add('hidden')
  document.getElementById('players').classList.remove('assembly')
  document.getElementById('progress-indicator').classList.remove('hidden')
}

let arenaBtns, handBtns, targetBtns
let selArena, selHand, selTarget

const updateGameplayIxnBtnsAndCheckAct = () => {
  // Send action message on match
  if (selArena !== undefined && selHand !== undefined && selTarget !== undefined) {
    send({
      type: 'action',
      hand_index: selHand,
      arena_index: selArena,
      target: selTarget,
    })
    selArena = undefined
    selHand = undefined
    selTarget = undefined
  }

  for (const [i, b] of Object.entries(arenaBtns)) {
    if (+i === selArena) b.classList.add('selected')
    else b.classList.remove('selected')
  }
  for (const [i, b] of Object.entries(handBtns)) {
    if (+i === selHand) b.classList.add('selected')
    else b.classList.remove('selected')
  }
  for (const [i, b] of Object.entries(targetBtns)) {
    if (+i === selTarget) b.classList.add('selected')
    else b.classList.remove('selected')
  }
}

const updateGameplayPanel = (gameplay_status) => {
  showGameplayPanel()
  document.getElementById('act-num').innerText = gameplay_status.act_count
  document.getElementById('round-num').innerText = gameplay_status.round_count

  let storyteller = undefined
  if (gameplay_status.step === 'storytelling_holder')
    storyteller = gameplay_status.holder
  else if (gameplay_status.step === 'storytelling_target')
    storyteller = gameplay_status.target
  markPlayer(gameplay_status.holder, storyteller)

  const elIxnContainer = document.getElementById('gameplay-interactions')
  if (gameplay_status.holder === myIndex && gameplay_status.step === 'selection')
    elIxnContainer.classList.add('active')
  else elIxnContainer.classList.remove('active')

  const elArena = document.getElementById('gameplay-arena-list')
  elArena.innerText = ''
  arenaBtns = []
  for (const [i, kw] of Object.entries(gameplay_status.arena)) {
    const node = document.createElement('button')
    node.innerText = kw
    elArena.appendChild(node)
    node.addEventListener('click', (e) => {
      selArena = +i
      updateGameplayIxnBtnsAndCheckAct()
    })
    arenaBtns.push(node)
  }

  const elHand = document.getElementById('gameplay-hand-list')
  elHand.innerText = ''
  handBtns = []
  for (const [i, card] of Object.entries(gameplay_status.hand)) {
    const node = document.createElement('button')
    node.innerText = card
    elHand.appendChild(node)
    node.addEventListener('click', (e) => {
      selHand = +i
      updateGameplayIxnBtnsAndCheckAct()
    })
    handBtns.push(node)

    const [requirements, _, relationshipChanges] = cardSet[card]
    const nodeDesc = document.createElement('span')
    nodeDesc.innerHTML = ` ${formatCogFns(requirements)} ${formatRelationship(relationshipChanges, true)}`
    elHand.appendChild(nodeDesc)

    elHand.appendChild(document.createElement('br'))
  }

  const elTarget = document.getElementById('gameplay-target-list')
  elTarget.innerText = ''
  targetBtns = []
  for (const [i, pf] of Object.entries(lastSavedPlayers)) {
    if (+i === myIndex) continue
    const node = document.createElement('button')
    node.innerText = `[${+i + 1}] ${pf.creator.nickname}`
    elTarget.appendChild(node)
    node.addEventListener('click', (e) => {
      selTarget = +i
      updateGameplayIxnBtnsAndCheckAct()
    })
    targetBtns[i] = node

    const nodeDesc = document.createElement('span')
    nodeDesc.innerHTML = ` — ${formatRelationship(gameplay_status.relationship[+i])}`
    elTarget.appendChild(nodeDesc)

    elTarget.appendChild(document.createElement('br'))
  }
  const node = document.createElement('button')
  node.innerText = '无特定对象'
  elTarget.appendChild(node)
  node.addEventListener('click', (e) => {
    selTarget = -1
    updateGameplayIxnBtnsAndCheckAct()
  })
  targetBtns[-1] = node

  if (storyteller === myIndex) {
    document.getElementById('storytelling-end').classList.remove('hidden')
    document.getElementById('storytelling-action').innerText = gameplay_status.action
    document.getElementById('storytelling-keyword').innerText = gameplay_status.arena[gameplay_status.keyword]
    document.getElementById('storytelling-holder-name').innerText =
      (gameplay_status.holder === myIndex ? '自己' :
       `[${gameplay_status.holder + 1}] ${lastSavedPlayers[gameplay_status.holder].creator.nickname}`)
    document.getElementById('storytelling-holder-result').innerText = resultText[gameplay_status.holder_result]
    if (gameplay_status.target !== null) {
      document.getElementById('storytelling-target-clause').classList.remove('hidden')
      document.getElementById('storytelling-target-name').innerText =
        (gameplay_status.target === myIndex ? '自己' :
         `[${gameplay_status.target + 1}] ${lastSavedPlayers[gameplay_status.target].creator.nickname}`)
      document.getElementById('storytelling-target-result').innerText = resultText[gameplay_status.target_result]
    } else {
      document.getElementById('storytelling-target-clause').classList.add('hidden')
    }
  } else {
    document.getElementById('storytelling-end').classList.add('hidden')
  }

  document.getElementById('queue-list').innerText =
    gameplay_status.queue.map((i) => `[${i + 1}] ${lastSavedPlayers[i].creator.nickname}`).join(', ')
  document.getElementById('btn-queue-join').disabled =
    !(gameplay_status.action_points > 0 && gameplay_status.holder !== myIndex
      && gameplay_status.queue.indexOf(myIndex) === -1)

  timerSet(gameplay_status.timer)
}

document.getElementById('btn-storytelling-end').addEventListener('click', (e) => {
  send({ type: 'storytelling_end' })
})

document.getElementById('btn-queue-join').addEventListener('click', (e) => {
  send({ type: 'queue' })
})

////// Game end panel //////

const showGameEndPanel = () => {
  document.getElementById('assembly-panel').classList.add('hidden')
  document.getElementById('appointment-panel').classList.add('hidden')
  document.getElementById('gameplay-panel').classList.add('hidden')
  document.getElementById('game-end-panel').classList.remove('hidden')
  document.getElementById('players').classList.remove('assembly')
  document.getElementById('progress-indicator').classList.add('hidden')

  markPlayer(-1)
  document.getElementById('storytelling-end').classList.add('hidden')
  timerSet(null)
}

const updateGameEndPanel = (o) => {
  showGameEndPanel()

  const elRelsContainer = document.getElementById('game-end-relationships')
  elRelsContainer.innerText = ''

  for (const [i, pf] of Object.entries(lastSavedPlayers)) {
    if (+i === myIndex) continue
    const node = document.createElement('span')
    node.innerHTML = `[${+i + 1}] ${pf.creator.nickname} — ${formatRelationship(o.relationship[+i])}`
    elRelsContainer.appendChild(node)
    elRelsContainer.appendChild(document.createElement('br'))
  }

  document.getElementById('game-end-growth').innerText = o.growth_points
}

/*
document.getElementById('btn-game-end-return').addEventListener('click', (e) => {
  updateAssemblyPanel(lastSavedPlayers.map((pf) => ({ id: null, creator: pf.creator })))
})
*/

// Connect after everything has been initialized;
// otherwise might receive `ReferenceError: can't access lexical declaration before initialization`
reconnect()

})()
