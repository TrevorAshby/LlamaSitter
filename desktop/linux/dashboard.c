#include "common.h"

#include <adwaita.h>
#include <gtk/gtk.h>

#if defined(LS_USE_WEBKITGTK_6)
#include <webkitgtk/webkitgtk.h>
#else
#include <webkit2/webkit2.h>
#endif

typedef struct {
    AdwApplication *application;
    LSRuntimeInfo runtime_info;
    gchar *self_executable;
    GtkWidget *window;
    GtkStack *stack;
    GtkLabel *loading_label;
    GtkLabel *error_label;
    GtkWidget *spinner;
    WebKitWebView *web_view;
    guint ready_source_id;
    gint remaining_attempts;
} LSDashboardApp;

static void ls_dashboard_show_loading(LSDashboardApp *state, const gchar *message) {
    gtk_label_set_text(state->loading_label, message);
    gtk_spinner_start(GTK_SPINNER(state->spinner));
    gtk_stack_set_visible_child_name(state->stack, "loading");
}

static void ls_dashboard_show_error(LSDashboardApp *state, const gchar *message) {
    gtk_spinner_stop(GTK_SPINNER(state->spinner));
    gtk_label_set_text(state->error_label, message);
    gtk_stack_set_visible_child_name(state->stack, "error");
}

static void ls_dashboard_show_web(LSDashboardApp *state) {
    gtk_spinner_stop(GTK_SPINNER(state->spinner));
    webkit_web_view_load_uri(state->web_view, state->runtime_info.ui_base_url);
    gtk_stack_set_visible_child_name(state->stack, "web");
}

static void ls_dashboard_ensure_tray(LSDashboardApp *state) {
    GError *error = NULL;
    if (!ls_spawn_mode(state->self_executable, "tray", state->runtime_info.config_path, TRUE, &error)) {
        gchar *message = g_strdup_printf("Unable to start the tray agent.\n\n%s",
                                         error != NULL ? error->message : "Unknown error.");
        ls_dashboard_show_error(state, message);
        g_clear_error(&error);
        g_free(message);
    }
}

static gboolean ls_dashboard_poll_ready(gpointer user_data) {
    LSDashboardApp *state = user_data;
    GError *error = NULL;

    if (ls_is_ready(state->runtime_info.ready_url, &error)) {
        ls_dashboard_show_web(state);
        state->ready_source_id = 0;
        return G_SOURCE_REMOVE;
    }
    g_clear_error(&error);

    state->remaining_attempts -= 1;
    if (state->remaining_attempts <= 0) {
        gchar *message = g_strdup_printf("The background LlamaSitter agent did not become ready within 20 seconds.\n\nCheck the app log at:\n%s",
                                         state->runtime_info.app_log_path != NULL ? state->runtime_info.app_log_path : "the desktop logs directory");
        ls_dashboard_show_error(state, message);
        g_free(message);
        state->ready_source_id = 0;
        return G_SOURCE_REMOVE;
    }

    return G_SOURCE_CONTINUE;
}

static void ls_dashboard_start_wait(LSDashboardApp *state) {
    if (state->ready_source_id != 0) {
        g_source_remove(state->ready_source_id);
        state->ready_source_id = 0;
    }

    state->remaining_attempts = 80;
    ls_dashboard_show_loading(state, "Starting the background agent if needed and waiting for metrics.");
    ls_dashboard_ensure_tray(state);
    state->ready_source_id = g_timeout_add(250, ls_dashboard_poll_ready, state);
}

static void ls_dashboard_retry(GtkButton *button, gpointer user_data) {
    LSDashboardApp *state = user_data;
    (void) button;
    ls_dashboard_start_wait(state);
}

static GtkWidget *ls_dashboard_build_loading_view(LSDashboardApp *state) {
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 16);
    GtkWidget *title = gtk_label_new("Opening Dashboard");

    state->spinner = gtk_spinner_new();
    gtk_spinner_start(GTK_SPINNER(state->spinner));

    state->loading_label = GTK_LABEL(gtk_label_new("Starting the background agent if needed and waiting for metrics."));
    gtk_label_set_wrap(state->loading_label, TRUE);
    gtk_widget_set_halign(title, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(state->spinner, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(GTK_WIDGET(state->loading_label), GTK_ALIGN_CENTER);
    gtk_widget_set_margin_top(box, 48);
    gtk_widget_set_margin_bottom(box, 48);
    gtk_widget_set_margin_start(box, 48);
    gtk_widget_set_margin_end(box, 48);
    gtk_box_append(GTK_BOX(box), state->spinner);
    gtk_box_append(GTK_BOX(box), title);
    gtk_box_append(GTK_BOX(box), GTK_WIDGET(state->loading_label));
    return box;
}

static GtkWidget *ls_dashboard_build_error_view(LSDashboardApp *state) {
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 16);
    GtkWidget *title = gtk_label_new("Unable to load metrics");
    GtkWidget *button = gtk_button_new_with_label("Retry");

    state->error_label = GTK_LABEL(gtk_label_new(""));
    gtk_label_set_wrap(state->error_label, TRUE);
    gtk_widget_set_halign(title, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(GTK_WIDGET(state->error_label), GTK_ALIGN_CENTER);
    gtk_widget_set_halign(button, GTK_ALIGN_CENTER);
    gtk_widget_set_margin_top(box, 48);
    gtk_widget_set_margin_bottom(box, 48);
    gtk_widget_set_margin_start(box, 48);
    gtk_widget_set_margin_end(box, 48);
    g_signal_connect(button, "clicked", G_CALLBACK(ls_dashboard_retry), state);
    gtk_box_append(GTK_BOX(box), title);
    gtk_box_append(GTK_BOX(box), GTK_WIDGET(state->error_label));
    gtk_box_append(GTK_BOX(box), button);
    return box;
}

static void ls_dashboard_activate(GApplication *application, gpointer user_data) {
    LSDashboardApp *state = user_data;
    GtkWidget *toolbar_view = NULL;
    GtkWidget *header = NULL;
    GtkWidget *title = NULL;

    if (state->window != NULL) {
        gtk_window_present(GTK_WINDOW(state->window));
        return;
    }

    state->window = adw_application_window_new(state->application);
    gtk_window_set_title(GTK_WINDOW(state->window), "LlamaSitter");
    gtk_window_set_default_size(GTK_WINDOW(state->window), 1220, 860);

    toolbar_view = adw_toolbar_view_new();
    header = adw_header_bar_new();
    title = adw_window_title_new("LlamaSitter", state->runtime_info.ui_listen_addr);
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(header), title);
    adw_toolbar_view_add_top_bar(ADW_TOOLBAR_VIEW(toolbar_view), header);

    state->stack = GTK_STACK(gtk_stack_new());
    gtk_stack_set_transition_type(state->stack, GTK_STACK_TRANSITION_TYPE_CROSSFADE);
    state->web_view = WEBKIT_WEB_VIEW(webkit_web_view_new());
    gtk_stack_add_named(state->stack, ls_dashboard_build_loading_view(state), "loading");
    gtk_stack_add_named(state->stack, ls_dashboard_build_error_view(state), "error");
    gtk_stack_add_named(state->stack, GTK_WIDGET(state->web_view), "web");
    adw_toolbar_view_set_content(ADW_TOOLBAR_VIEW(toolbar_view), GTK_WIDGET(state->stack));

    gtk_window_set_child(GTK_WINDOW(state->window), toolbar_view);
    gtk_window_present(GTK_WINDOW(state->window));
    ls_dashboard_start_wait(state);

    (void) application;
}

static void ls_dashboard_shutdown(GApplication *application, gpointer user_data) {
    LSDashboardApp *state = user_data;
    if (state->ready_source_id != 0) {
        g_source_remove(state->ready_source_id);
        state->ready_source_id = 0;
    }
    ls_runtime_info_clear(&state->runtime_info);
    g_clear_pointer(&state->self_executable, g_free);
    (void) application;
}

int ls_dashboard_run(int argc, char **argv, const gchar *self_executable, const gchar *config_override, gboolean attach_only) {
    LSDashboardApp state = {0};
    GError *error = NULL;
    gchar *cli_executable = NULL;
    gchar *application_id = NULL;
    int status = 1;

    adw_init();
    state.self_executable = g_strdup(self_executable);
    cli_executable = ls_find_cli_executable(self_executable);
    if (!ls_runtime_info_load(cli_executable, config_override, attach_only, &state.runtime_info, &error)) {
        g_printerr("%s\n", error != NULL ? error->message : "Unable to resolve the desktop runtime.");
        g_clear_error(&error);
        g_clear_pointer(&cli_executable, g_free);
        ls_runtime_info_clear(&state.runtime_info);
        g_clear_pointer(&state.self_executable, g_free);
        return 1;
    }
    g_clear_pointer(&cli_executable, g_free);

    application_id = ls_application_id_for_config("com.trevorashby.LlamaSitter", state.runtime_info.config_path);
    state.application = ADW_APPLICATION(adw_application_new(application_id, G_APPLICATION_DEFAULT_FLAGS));
    g_free(application_id);

    g_signal_connect(state.application, "activate", G_CALLBACK(ls_dashboard_activate), &state);
    g_signal_connect(state.application, "shutdown", G_CALLBACK(ls_dashboard_shutdown), &state);
    status = g_application_run(G_APPLICATION(state.application), argc, argv);
    g_object_unref(state.application);
    return status;
}
