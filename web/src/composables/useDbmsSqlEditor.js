import { computed, nextTick, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { at } from '../utils/asset-i18n'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 85-97 editor state·favorites · 164-176 keyword pool·snippets ·
// 201-209 sqlSnippets · 230-251 autocomplete · 253-254 highlight·lines ·
// 272-330 escape·highlight·favorites storage·format · 332-355 save·apply·
// remove favorite · 421-428 syncEditorMetrics · 746-751 selectedSQLText ·
// 1028-1090 snippet insert·autocomplete·editor keydown.
// Rewiring notes: sqlText is owned here; runSQL arrives late via `bridges`
// (ctrl+enter binding — the execution composable is created after this one,
// same late-wiring pattern as useK8sClusterState's load hooks).
export function useDbmsSqlEditor({ databaseId, isPostgres, schemaTree, selectedColumns, bridges }) {
  const showSuggestions = ref(false)
  const currentToken = ref('')
  const activeSuggestionIndex = ref(0)
  const currentLine = ref(1)
  const sqlText = ref('SELECT *\nFROM your_table\nLIMIT 50;')
  const sqlScrollTop = ref(0)
  const sqlScrollLeft = ref(0)
  const sqlEditorRef = ref(null)
  const editorWrapRef = ref(null)
  const sqlFavorites = ref([])

  const sqlKeywordPool = [
    'SELECT', 'FROM', 'WHERE', 'ORDER BY', 'GROUP BY', 'HAVING', 'LIMIT', 'OFFSET',
    'INSERT INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE FROM', 'CREATE TABLE', 'ALTER TABLE',
    'DROP TABLE', 'SHOW TABLES', 'SHOW DATABASES', 'DESCRIBE', 'EXPLAIN', 'LEFT JOIN',
    'RIGHT JOIN', 'INNER JOIN', 'UNION ALL', 'COUNT', 'SUM', 'MIN', 'MAX', 'AVG', 'NOW()'
  ]

  const baseSqlSnippets = [
    { label: 'SELECT', text: 'SELECT *\nFROM table_name\nLIMIT 50;' },
    { label: 'UPDATE', text: "UPDATE table_name\nSET column_name = 'value'\nWHERE id = 1;" },
    { label: 'INSERT', text: "INSERT INTO table_name (column_1, column_2)\nVALUES ('value_1', 'value_2');" },
    { label: 'DELETE', text: 'DELETE FROM table_name\nWHERE id = 1;' }
  ]

  const sqlSnippets = computed(() => [
    ...baseSqlSnippets,
    isPostgres.value
      ? {
          label: at('viewTablesLabel'),
          text: "SELECT table_schema, table_name\nFROM information_schema.tables\nWHERE table_type = 'BASE TABLE'\n  AND table_schema NOT IN ('pg_catalog', 'information_schema')\n  AND table_schema NOT LIKE 'pg_%'\nORDER BY table_schema, table_name;"
        }
      : { label: 'SHOW TABLES', text: 'SHOW TABLES;' }
  ])

  const allSuggestionItems = computed(() => {
    const pool = new Set(sqlKeywordPool)
    for (const schema of schemaTree.value) {
      pool.add(schema.name)
      for (const table of schema.tables || []) {
        pool.add(table.name)
        pool.add(`${schema.name}.${table.name}`)
      }
    }
    for (const col of selectedColumns.value) {
      pool.add(col.name)
    }
    return Array.from(pool)
  })

  const suggestions = computed(() => {
    const keyword = currentToken.value.trim().toLowerCase()
    if (!keyword) return []
    return allSuggestionItems.value
      .filter((item) => item.toLowerCase().includes(keyword))
      .slice(0, 14)
  })

  const highlightedSQL = computed(() => highlightSQL(sqlText.value))
  const sqlLines = computed(() => Array.from({ length: Math.max(sqlText.value.split('\n').length, 1) }, (_, index) => index + 1))

  function escapeHTML(value) {
    return String(value)
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
  }

  function highlightSQL(source) {
    let html = escapeHTML(source)
    html = html.replace(/(--.*)$/gm, '<span class="token-comment">$1</span>')
    html = html.replace(/('(?:''|[^'])*')/g, '<span class="token-string">$1</span>')
    html = html.replace(/\b(\d+)\b/g, '<span class="token-number">$1</span>')
    html = html.replace(/\b(SELECT|FROM|WHERE|ORDER BY|GROUP BY|HAVING|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE FROM|CREATE TABLE|ALTER TABLE|DROP TABLE|SHOW|TABLES|DATABASES|DESCRIBE|EXPLAIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|UNION ALL|COUNT|SUM|AVG|MIN|MAX|NOW)\b/gi, '<span class="token-keyword">$1</span>')
    return html
  }

  function sqlFavoritesStorageKey() {
    return `ops-admin.dbms.sql-favorites.${databaseId.value}`
  }

  function loadSqlFavorites() {
    try {
      const saved = JSON.parse(localStorage.getItem(sqlFavoritesStorageKey()) || '[]')
      sqlFavorites.value = Array.isArray(saved) ? saved.filter((item) => item?.name && item?.sqlText).slice(0, 30) : []
    } catch {
      sqlFavorites.value = []
    }
  }

  function persistSqlFavorites() {
    localStorage.setItem(sqlFavoritesStorageKey(), JSON.stringify(sqlFavorites.value.slice(0, 30)))
  }

  function basicFormatSQL(source) {
    const parts = String(source || '').split(/('(?:''|[^'])*'|"(?:""|[^"])*")/g)
    return parts.map((part, index) => {
      if (index % 2) return part
      return part
        .replace(/\s+/g, ' ')
        .replace(/\b(SELECT|FROM|WHERE|GROUP BY|ORDER BY|HAVING|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE FROM|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|UNION ALL|CREATE TABLE|ALTER TABLE|DROP TABLE)\b/gi, (_, keyword) => `\n${keyword.toUpperCase()}`)
        .replace(/\s*,\s*/g, ', ')
    }).join('').replace(/^\s+|\s+$/g, '').replace(/;\s*(?=\S)/g, ';\n\n')
  }

  function formatSQL() {
    const editor = sqlEditorRef.value
    const source = selectedSQLText()
    if (!source) return ElMessage.warning(at('enterSQLToFormat'))
    const formatted = basicFormatSQL(source)
    if (editor && editor.selectionStart !== editor.selectionEnd) {
      const before = sqlText.value.slice(0, editor.selectionStart)
      const after = sqlText.value.slice(editor.selectionEnd)
      sqlText.value = before + formatted + after
      nextTick(() => editor.setSelectionRange(before.length, before.length + formatted.length))
    } else {
      sqlText.value = formatted
    }
    ElMessage.success(at('sqlFormatDone'))
  }

  async function saveCurrentSQL() {
    const statement = selectedSQLText()
    if (!statement) return ElMessage.warning(at('enterSQLToSave'))
    const { value } = await ElMessageBox.prompt(at('sqlFavoritePrompt'), 'SQL Favorite', {
      inputPlaceholder: at('sqlFavoriteNamePlaceholder'),
      inputPattern: /\S+/,
      inputErrorMessage: at('sqlFavoriteNameRequired'),
      confirmButtonText: at('save'),
      cancelButtonText: at('cancel')
    })
    sqlFavorites.value.unshift({ id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`, name: value.trim(), sqlText: statement, updatedAt: Date.now() })
    persistSqlFavorites()
    ElMessage.success(at('sqlFavoriteSaved'))
  }

  function applySqlFavorite(item) {
    sqlText.value = item.sqlText
    nextTick(() => sqlEditorRef.value?.focus())
  }

  function removeSqlFavorite(item) {
    sqlFavorites.value = sqlFavorites.value.filter((current) => current.id !== item.id)
    persistSqlFavorites()
  }

  function syncEditorMetrics() {
    const el = sqlEditorRef.value
    if (!el) return
    sqlScrollTop.value = el.scrollTop
    sqlScrollLeft.value = el.scrollLeft
    const before = sqlText.value.slice(0, el.selectionStart)
    currentLine.value = before.split('\n').length
  }

  function selectedSQLText() {
    const editor = sqlEditorRef.value
    if (!editor) return sqlText.value.trim()
    const selected = sqlText.value.slice(editor.selectionStart, editor.selectionEnd).trim()
    return selected || sqlText.value.trim()
  }

  function insertSnippet(text) {
    sqlText.value = text
    nextTick(() => {
      sqlEditorRef.value?.focus()
      syncEditorMetrics()
    })
  }

  function updateAutocomplete() {
    const editor = sqlEditorRef.value
    if (!editor) return
    const before = sqlText.value.slice(0, editor.selectionStart)
    const match = before.match(/[A-Za-z_][A-Za-z0-9_.]*$/)
    currentToken.value = match ? match[0] : ''
    activeSuggestionIndex.value = 0
    showSuggestions.value = !!currentToken.value && suggestions.value.length > 0
    syncEditorMetrics()
  }

  function applySuggestion(value) {
    const editor = sqlEditorRef.value
    if (!editor) return
    const cursor = editor.selectionStart
    const before = sqlText.value.slice(0, cursor)
    const after = sqlText.value.slice(cursor)
    const replacedBefore = before.replace(/[A-Za-z_][A-Za-z0-9_.]*$/, value)
    sqlText.value = replacedBefore + after
    showSuggestions.value = false
    nextTick(() => {
      editor.focus()
      const pos = replacedBefore.length
      editor.setSelectionRange(pos, pos)
      syncEditorMetrics()
    })
  }

  function onEditorKeydown(event) {
    if (showSuggestions.value && suggestions.value.length) {
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        activeSuggestionIndex.value = (activeSuggestionIndex.value + 1) % suggestions.value.length
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        activeSuggestionIndex.value = (activeSuggestionIndex.value - 1 + suggestions.value.length) % suggestions.value.length
        return
      }
      if (event.key === 'Tab' || event.key === 'Enter') {
        event.preventDefault()
        applySuggestion(suggestions.value[activeSuggestionIndex.value])
        return
      }
      if (event.key === 'Escape') {
        showSuggestions.value = false
        return
      }
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'enter') {
      event.preventDefault()
      // 재배선 — runSQL은 실행 도메인(useDbmsSqlExecution) 소관. 생성 순서상
      // 브릿지로 늦게 주입된다(원본 1086-1089의 직접 호출과 동일 동작).
      bridges?.runSQL?.()
    }
  }

  return {
    showSuggestions,
    currentToken,
    activeSuggestionIndex,
    currentLine,
    sqlText,
    sqlScrollTop,
    sqlScrollLeft,
    sqlEditorRef,
    editorWrapRef,
    sqlFavorites,
    sqlSnippets,
    suggestions,
    highlightedSQL,
    sqlLines,
    loadSqlFavorites,
    formatSQL,
    saveCurrentSQL,
    applySqlFavorite,
    removeSqlFavorite,
    syncEditorMetrics,
    selectedSQLText,
    insertSnippet,
    updateAutocomplete,
    applySuggestion,
    onEditorKeydown
  }
}
