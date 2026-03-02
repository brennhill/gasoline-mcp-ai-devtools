package schema

// interactActions is the canonical list of values accepted by the 'what' parameter.
// The deprecated 'action' alias references this same slice — do not mutate it at runtime.
var interactActions = []string{
	"highlight", "subtitle", "save_state", "state_save", "load_state", "state_load", "list_states", "state_list", "delete_state", "state_delete",
	"set_storage", "delete_storage", "clear_storage", "set_cookie", "delete_cookie",
	"execute_js", "navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab", "screenshot",
	"click", "type", "select", "check",
	"get_text", "get_value", "get_attribute", "query",
	"set_attribute", "focus", "scroll_to", "wait_for", "key_press", "paste",
	"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
	"hover",
	"auto_dismiss_overlays",
	"wait_for_stable",
	"list_interactive",
	"get_readable", "get_markdown",
	"navigate_and_wait_for", "fill_form_and_submit", "fill_form", "run_a11y_and_export_sarif",
	"record_start", "record_stop",
	"upload", "draw_mode_start",
	"hardware_click", "activate_tab",
	"explore_page",
	"batch",
	"clipboard_read", "clipboard_write",
}
