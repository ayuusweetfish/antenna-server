(async () => {

const api = 'http://localhost:10405'
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
const relAspect = ['△激情', '○亲密', '□责任']

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
      updateAppointmentPanel({ holder: o.holder, timer: 60 })
    } else if (o.type === 'appointment_pass') {
      updateAppointmentPanel({ holder: o.next_holder, timer: 60 })
    } else if (o.type === 'appointment_accept') {
      updateGameplayPanel(o.gameplay_status)
    } else if (o.type === 'gameplay_progress') {
      updateGameplayPanel(o.gameplay_status)
    } else if (o.type === 'game_end') {
      showGameEndPanel()
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
      (pf.id === null ? '[not seated]' : `[${+i + 1}]`) +
      ` ${htmlEscape(pf.creator.nickname)}` +
      (pf.id === null ? '' : ` (${htmlEscape(pf.details.race)}; ${formatStats(pf.stats)})`)
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
  stats.map((n, i) => `<span class='cog-fn-${cogFn[i]}'>${cogFn[i]} <strong>${n}</strong></span>`).join(', ')
const formatCogFns = (fns) =>
  fns.map((i) => `<span class='cog-fn-${cogFn[i]}'>${cogFn[i]}</span>`).join(' ')
const formatRelationship = (rel, plus) =>
  rel.map((n, i) => `<span class='rel-aspect-${i}'>${relAspect[i]} <strong>${plus && n > 0 ? '+' : ''}${n}</strong></span>`).join(', ')

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

////// Assembly panel //////

const profileRepr = (pf, omitId) => `${omitId ? '' : `[${pf.id}]: `}race <strong>${htmlEscape(pf.details.race)}</strong>, descr "<strong>${htmlEscape(pf.details.description)}</strong>", ${formatStats(pf.stats)}`

const showAssemblyPanel = () => {
  document.getElementById('assembly-panel').classList.remove('hidden')
  document.getElementById('appointment-panel').classList.add('hidden')
  document.getElementById('gameplay-panel').classList.add('hidden')
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
  node.innerHTML = `Seat with profile [${pf.id}]`
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

for (let i = 1; i <= 8; i++) {
  const el = document.getElementById(`stat-${i}`)
  const elValue = document.getElementById(`value-stat-${i}`)
  el.addEventListener('input', (e) => {
    elValue.innerText = el.value
  })
  elValue.innerText = el.value
}
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
    nodeDesc.innerHTML = ` — ${formatCogFns(requirements)} — ${formatRelationship(relationshipChanges, true)}`
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
  node.innerText = 'No target'
  elTarget.appendChild(node)
  node.addEventListener('click', (e) => {
    selTarget = -1
    updateGameplayIxnBtnsAndCheckAct()
  })
  targetBtns[-1] = node

  if (storyteller === myIndex)
    document.getElementById('storytelling-end').classList.remove('hidden')
  else document.getElementById('storytelling-end').classList.add('hidden')

  document.getElementById('queue-list').innerText =
    gameplay_status.queue.map((i) => `[${i + 1}] ${lastSavedPlayers[i].creator.nickname}`).join(', ')
  document.getElementById('btn-queue-join').disabled =
    !(gameplay_status.action_points > 0 && gameplay_status.holder !== myIndex
      && gameplay_status.queue.indexOf(myIndex) === -1)
}

document.getElementById('btn-storytelling-end').addEventListener('click', (e) => {
  send({ type: 'storytelling_end' })
})

document.getElementById('btn-queue-join').addEventListener('click', (e) => {
  send({ type: 'queue' })
})

document.getElementById('btn-comment').addEventListener('click', (e) => {
  send({ type: 'comment', text: document.getElementById('txt-comment').value })
  document.getElementById('txt-comment').value = ''
})

////// Game end panel //////

const showGameEndPanel = () => {
  markPlayer(-1)
  document.getElementById('storytelling-end').classList.add('hidden')
}

// Connect after everything has been initialized;
// otherwise might receive `ReferenceError: can't access lexical declaration before initialization`
reconnect()

})()
