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
window.history.replaceState(null, null, `?uid=${uid}&room=${rid}`)

const user = await (await fetch(`${api}/me`, { credentials: 'include' })).json()
const room = await (await fetch(`${api}/room/${rid}`, { credentials: 'include' })).json()

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
    error('Connection dropped')
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
      if (o.phase === 'assembly') {
        updateAssemblyPanel(o.players)
      } else if (o.phase === 'appointment') {
      } else if (o.phase === 'gameplay') {
      }
    } else if (o.type === 'assembly_update') {
      updateAssemblyPanel(o.players)
    }
  }
}
reconnect()

const send = (o) => ws.send(JSON.stringify(o))

const htmlEscape = (s) =>
  s.replaceAll('&', '&amp;')
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
}

const profileRepr = (pf) => `[${pf.id}]: race ${pf.details.race}, desc ${pf.details.description}, stats ${pf.stats.join(', ')}`

const showAssemblyPanel = () => {
  document.getElementById('assembly-panel').classList.remove('hidden')
  document.getElementById('gameplay-panel').classList.add('hidden')
}
const showSeatAndProfiles = () => {
  document.getElementById('seat-profiles').classList.remove('hidden')
  document.getElementById('seat-withdraw').classList.add('hidden')
}
const showSeatWithdraw = (p) => {
  document.getElementById('seat-profiles').classList.add('hidden')
  document.getElementById('seat-withdraw').classList.remove('hidden')
  document.getElementById('seated-profile').innerText = profileRepr(p)
}
const updateAssemblyPanel = (players) => {
  showAssemblyPanel()
  const p = players.find((p) => p.creator.id === uid && p.id !== null)
  if (p)
    showSeatWithdraw(p)
  else
    showSeatAndProfiles()
}

const addProfileButton = (pf) => {
  const elContainer = document.getElementById('profiles')
  const node = document.createElement('button')
  node.innerText = `Seat with profile ${profileRepr(pf)}`
  node.addEventListener('click', (e) => {
    send({
      type: 'seat',
      profile_id: pf.id,
    })
  })
  elContainer.appendChild(node)
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

})()
