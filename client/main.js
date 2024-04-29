(async () => {

const api = 'http://localhost:10405'
const form = (dict) => {
  const d = new FormData()
  for (const [k, v] of Object.entries(dict))
    d.set(k, v)
  return d
}

const uid = +(window.location.search ? window.location.search.substring(1) : prompt('uid'))
const headers = new Headers({
  'Authorization': `Bearer !${uid}`
})
console.log(headers)

const user = await (await fetch(`${api}/me`, { headers })).json()
console.log(user)

document.getElementById('uid').innerText = uid
document.getElementById('nickname').innerText = user.nickname

})()
