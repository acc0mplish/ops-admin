export function buildTree(list, idKey = 'id', parentKey = 'parentId') {
  const map = new Map()
  const roots = []

  list.forEach((item) => {
    map.set(item[idKey], { ...item, children: [] })
  })

  map.forEach((node) => {
    const parentId = node[parentKey]
    if (!parentId || !map.has(parentId)) {
      roots.push(node)
      return
    }
    map.get(parentId).children.push(node)
  })

  return roots
}
