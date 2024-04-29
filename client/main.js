(async () => {

const api = 'http://localhost:10405'
const apiUrl = new URL(api)
const form = (dict) => {
  const d = new FormData()
  for (const [k, v] of Object.entries(dict))
    d.set(k, v)
  return d
}

const search = new URL(document.location).searchParams
const uid = +(search.get('uid') || prompt('User ID'))
const rid = +(search.get('room') || prompt('Room ID'))
document.cookie = `auth=!${uid}; SameSite=Lax; Path=/; Secure; Max-Age=604800`

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
    }
  }
}
reconnect()

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

})()
