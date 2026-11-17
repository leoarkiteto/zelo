package i18n

// Pagination area: the shared list pagination navigation (pagination.*).
func init() {
	registerArea("pagination", enPagination, ptbrPagination, ptptPagination)
}

var enPagination = map[MessageKey]string{
	"pagination.label":    "Pagination",
	"pagination.page_of":  "Page %d of %d",
	"pagination.previous": "Previous",
	"pagination.next":     "Next",
}

var ptbrPagination = map[MessageKey]string{
	"pagination.label":    "Paginação",
	"pagination.page_of":  "Página %d de %d",
	"pagination.previous": "Anterior",
	"pagination.next":     "Próxima",
}

var ptptPagination = map[MessageKey]string{
	"pagination.label":    "Paginação",
	"pagination.page_of":  "Página %d de %d",
	"pagination.previous": "Anterior",
	"pagination.next":     "Seguinte",
}
