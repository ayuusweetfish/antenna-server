// Populates the database for testing
// Check the `/data` endpoint for a dump of all data

// deno run --allow-net %
// bun %

// const api = 'https://antenna-nightly.0-th.art'
const api = 'http://0.0.0.0:10405'
const form = (dict) => {
  const d = new URLSearchParams()
  for (const [k, v] of Object.entries(dict))
    d.set(k, v)
  return d
}

// Users
const major_arcana = ['愚者', '魔法师', '女祭司', '皇后', '皇帝', '教皇', '恋人', '战车', '力量', '隐士', '命运之轮', '正义', '倒吊人', '死神', '节制', '恶魔', '高塔', '星星', '月亮', '太阳', '审判', '世界']
for (let i = 1; i <= 22; i++) {
  console.log(await (await fetch(`${api}/sign-up`, {
    method: 'POST',
    body: form({
      nickname: major_arcana[i % 22],
      password: '111',
    }),
  })).text())
}

let seed = 2024051
const rand = (n) => {
  seed = (seed * 1664525 + 1013904223) | 0
  if (seed < 0) seed += 4294967296
  return (n === undefined ? seed : seed % n)
}

// Profiles
const archetype = {
  INFP: 'Mediator',
  INFJ: 'Advocate',
  INTP: 'Logician',
  INTJ: 'Architect',
  ENFP: 'Campaigner',
  ENFJ: 'Protagonist',
  ENTP: 'Debater',
  ENTJ: 'Commander',
  ISFP: 'Adventurer',
  ISFJ: 'Defender',
  ISTP: 'Virtuoso',
  ISTJ: 'Logistician',
  ESFP: 'Entertainer',
  ESFJ: 'Consul',
  ESTP: 'Entrepreneur',
  ESTJ: 'Executive',
}
for (let i = 1; i <= 22; i++) {
  for (let j = 0; j < 5; j++) {
    const values = []
    do {
      const samples = new Set()
      for (let i = 400; i < 407; i++) {
        let x = rand(i + 1)
        if (samples.has(x)) x = i
        samples.add(x)
      }
      const sorted = [-1, 407, ...samples.values()].sort((a, b) => a - b)
      for (let i = 0; i < 8; i++)
        values[i] = sorted[i + 1] - sorted[i] - 1 + 10
    } while (!values.every((n) => n <= 90))

    // Find largest
    let dom = 0
    for (let i = 1; i < 8; i++)
      if (values[i] > values[dom]) dom = i
    const aux1 = dom ^ 5
    const aux2 = dom ^ 7
    const aux = (values[aux1] >= values[aux2] ? aux1 : aux2)
    const ie = (dom % 2 === 0 ? 'E' : 'I')
    const ns = (Math.min(dom, aux) < 2 ? 'S' : 'N')
    const tf = (Math.max(dom, aux) >= 6 ? 'F' : 'T')
    const jp = (Math.max(dom, aux) % 2 === 0 ? 'J' : 'P')

    const race = ie + ns + tf + jp
    console.log(await (await fetch(`${api}/profile/create`, {
      method: 'POST',
      headers: {
        'Cookie': `auth=!${i}`,
      },
      body: form({
        details: JSON.stringify({
          gender: ['female', 'male', 'non-binary'][rand(3)],
          orientation: ['hetero', 'homo', 'ace', 'pan', 'omni', 'demi'][rand(6)],
          race: race,
          age: 10 + rand(90),
          description: `The ${archetype[race]}`,
        }),
        stats: values.join(','),
        traits: '',
      }),
    })).text())
  }
}

// Rooms
const qianziwen = [
  '天地玄黄', '宇宙洪荒',
  '日月盈昃', '辰宿列张',
  '寒来暑往', '秋收冬藏',
  '闰馀成岁', '律吕调阳',
  '云腾致雨', '露结为霜',
  '金生丽水', '玉出昆冈',
  '剑号巨阙', '珠称夜光',
  '果珍李柰', '菜重芥姜',
  '海咸河淡', '鳞潜羽翔',
  '龙师火帝', '鸟官人皇',
  '始制文字', '乃服衣裳',
]
for (let i = 1; i <= 22; i++) {
  console.log(await (await fetch(`${api}/room/create`, {
    method: 'POST',
    headers: {
      'Cookie': `auth=!${i}`,
    },
    body: form({
      title: qianziwen[i - 1],
      tags: `tag1,tag2`,
      description: `${major_arcana[i % 22]}的房间`,
    }),
  })).text())
}
