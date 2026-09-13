import { computed, reactive, ref, watch } from 'vue'
import { queryDBMSSchemaTree } from '../api/dbms'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 58 schemaTree · 80-84 tree selection·keyword·pages · 95 treeRef ·
// 38 treeLoading · 376-393 normalizeTree · 395-419 pagedSchemaTree ·
// 430-437 treeFilterMethod·changeSchemaTablePage · 492-504 loadTree ·
// 1268-1270 keyword watch. Free variables injected: databaseId·selectedSchema.
export function useDbmsSchemaTree({ databaseId, selectedSchema }) {
  const treeLoading = ref(false)
  const schemaTree = ref([])
  const treeKeyword = ref('')
  const schemaTablePages = reactive({})
  const schemaTablePageSize = 20
  const treeRef = ref(null)

  function normalizeTree(data) {
    return (data.schemas || []).map((schema) => ({
      id: `schema:${schema.name}`,
      label: schema.name,
      name: schema.name,
      isSchema: true,
      tableCount: Number(schema.tableCount || schema.tables?.length || 0),
      tables: schema.tables || [],
      children: (schema.tables || []).map((table) => ({
        id: `table:${schema.name}.${table.name}`,
        label: table.name,
        name: table.name,
        schema: schema.name,
        rows: table.rows,
        isTable: true
      }))
    }))
  }

  const pagedSchemaTree = computed(() => {
    const keyword = treeKeyword.value.trim().toLowerCase()
    return schemaTree.value
      .map((schema) => {
        const allChildren = schema.children || []
        const schemaMatched = schema.name.toLowerCase().includes(keyword)
        const children = keyword && !schemaMatched
          ? allChildren.filter((table) => {
            const fullName = `${schema.name}.${table.name}`.toLowerCase()
            return String(table.name || '').toLowerCase().includes(keyword) || fullName.includes(keyword)
          })
          : allChildren
        const totalPages = Math.max(1, Math.ceil(children.length / schemaTablePageSize))
        const currentPage = Math.min(Math.max(Number(schemaTablePages[schema.name] || 1), 1), totalPages)
        const start = (currentPage - 1) * schemaTablePageSize
        return {
          ...schema,
          children: children.slice(start, start + schemaTablePageSize),
          visibleTableCount: children.length,
          currentPage,
          totalPages
        }
      })
      .filter((schema) => !keyword || schema.children.length > 0 || schema.name.toLowerCase().includes(keyword))
  })

  function treeFilterMethod() {
    return true
  }

  function changeSchemaTablePage(schema, page) {
    const nextPage = Math.min(Math.max(page, 1), schema.totalPages || 1)
    schemaTablePages[schema.name] = nextPage
  }

  async function loadTree() {
    treeLoading.value = true
    try {
      const data = await queryDBMSSchemaTree(databaseId.value)
      schemaTree.value = normalizeTree(data)
      Object.keys(schemaTablePages).forEach((key) => delete schemaTablePages[key])
      if (!selectedSchema.value && data.defaultSchema) {
        selectedSchema.value = data.defaultSchema
      }
    } finally {
      treeLoading.value = false
    }
  }

  watch(treeKeyword, () => {
    Object.keys(schemaTablePages).forEach((key) => delete schemaTablePages[key])
  })

  return {
    treeLoading,
    schemaTree,
    treeKeyword,
    schemaTablePages,
    schemaTablePageSize,
    treeRef,
    pagedSchemaTree,
    normalizeTree,
    treeFilterMethod,
    changeSchemaTablePage,
    loadTree
  }
}
