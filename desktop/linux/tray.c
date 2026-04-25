#include "common.h"

#include <gtk/gtk.h>

#if defined(LS_HAVE_APPINDICATOR)
#if defined(LS_USE_AYATANA_APPINDICATOR)
#include <libayatana-appindicator/app-indicator.h>
#else
#include <libappindicator/app-indicator.h>
#endif
#endif

typedef struct {
    GtkApplication *application;
    LSRuntimeInfo runtime_info;
    gchar *self_executable;
    GSubprocess *backend_process;
    gboolean backend_owned;
    gboolean attached_to_external_service;
    gboolean intentionally_stopping;
    LSServiceStatus status;
    gchar *message;
    guint refresh_source_id;
    GtkWidget *menu;

#if defined(LS_HAVE_APPINDICATOR)
    AppIndicator *indicator;
#endif

    GtkWidget *status_item;
    GtkWidget *updated_item;
    GtkWidget *requests_item;
    GtkWidget *tokens_item;
    GtkWidget *duration_item;
    GtkWidget *model_item;
    GtkWidget *instance_item;
    GtkWidget *activity_title_item;
    GtkWidget *activity_detail_item;
    GtkWidget *retry_item;
    GtkWidget *fallback_window;
} LSTrayApp;

static void ls_tray_set_message(LSTrayApp *state, const gchar *message) {
    g_clear_pointer(&state->message, g_free);
    state->message = g_strdup(message != NULL ? message : "");
}

static gchar *ls_tray_metric_label(const gchar *name, const gchar *value) {
    if (name == NULL || *name == '\0') {
        return g_strdup(value != NULL && *value != '\0' ? value : "No data yet");
    }
    return g_strdup_printf("%s: %s", name, value != NULL && *value != '\0' ? value : "No data yet");
}

static void ls_tray_open_dashboard_button(GtkButton *button, gpointer user_data) {
    LSTrayApp *state = user_data;
    GError *error = NULL;
    (void) button;

    if (!ls_spawn_mode(state->self_executable, "dashboard", state->runtime_info.config_path, FALSE, &error)) {
        g_warning("Unable to open dashboard: %s", error != NULL ? error->message : "unknown error");
        g_clear_error(&error);
    }
}

static void ls_tray_update_menu(LSTrayApp *state, const LSDesktopOverview *overview) {
    gchar *label = NULL;
    gchar *duration = NULL;

    label = g_strdup_printf("Status: %s", ls_status_label(state->status));
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->status_item), label);
    g_free(label);

    label = g_strdup_printf("Updated: %s",
                            overview != NULL && overview->last_refresh_at != NULL ? overview->last_refresh_at :
                            (state->message != NULL && *state->message != '\0' ? state->message : "Waiting for the local API"));
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->updated_item), label);
    g_free(label);

    label = g_strdup_printf("Requests: %" G_GINT64_FORMAT, overview != NULL ? overview->request_count : 0);
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->requests_item), label);
    g_free(label);

    label = g_strdup_printf("Total Tokens: %" G_GINT64_FORMAT, overview != NULL ? overview->total_tokens : 0);
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->tokens_item), label);
    g_free(label);

    duration = g_strdup_printf("%.0f ms", overview != NULL ? overview->average_request_duration_ms : 0.0);
    label = ls_tray_metric_label("Average Duration", duration);
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->duration_item), label);
    g_free(label);
    g_free(duration);

    label = ls_tray_metric_label("Top Model", overview != NULL ? overview->top_model : NULL);
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->model_item), label);
    g_free(label);

    label = ls_tray_metric_label("Top Instance", overview != NULL ? overview->top_client_instance : NULL);
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->instance_item), label);
    g_free(label);

    label = ls_tray_metric_label("Latest Activity", overview != NULL ? overview->activity_title : "No activity yet");
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->activity_title_item), label);
    g_free(label);

    label = ls_tray_metric_label("", overview != NULL ? overview->activity_detail : "Requests and sessions will appear here once the proxy captures traffic.");
    gtk_menu_item_set_label(GTK_MENU_ITEM(state->activity_detail_item), label);
    g_free(label);

    gtk_widget_set_sensitive(state->retry_item,
                             state->status == LS_SERVICE_STATUS_FAILED || state->status == LS_SERVICE_STATUS_IDLE);

#if defined(LS_HAVE_APPINDICATOR)
    if (state->indicator != NULL) {
        app_indicator_set_status(state->indicator,
                                 state->status == LS_SERVICE_STATUS_FAILED ? APP_INDICATOR_STATUS_ATTENTION :
                                 APP_INDICATOR_STATUS_ACTIVE);
    }
#endif
}

static void ls_tray_show_fallback_window(LSTrayApp *state) {
    GtkWidget *box = NULL;
    GtkWidget *title = NULL;
    GtkWidget *detail = NULL;
    GtkWidget *button_row = NULL;
    GtkWidget *open_button = NULL;
    GtkWidget *quit_button = NULL;

    if (state->fallback_window != NULL) {
        gtk_window_present(GTK_WINDOW(state->fallback_window));
        return;
    }

    state->fallback_window = gtk_application_window_new(state->application);
    gtk_window_set_title(GTK_WINDOW(state->fallback_window), "LlamaSitter Tray");
    gtk_window_set_default_size(GTK_WINDOW(state->fallback_window), 420, 180);

    box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_margin_top(box, 24);
    gtk_widget_set_margin_bottom(box, 24);
    gtk_widget_set_margin_start(box, 24);
    gtk_widget_set_margin_end(box, 24);
    title = gtk_label_new("Tray unavailable on this desktop");
    detail = gtk_label_new("LlamaSitter will keep the backend available, but no StatusNotifier host was found. Open the dashboard for visibility or quit the tray agent.");
    gtk_label_set_line_wrap(GTK_LABEL(detail), TRUE);
    button_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
    open_button = gtk_button_new_with_label("Open Dashboard");
    quit_button = gtk_button_new_with_label("Quit");
    gtk_box_pack_start(GTK_BOX(button_row), open_button, TRUE, TRUE, 0);
    gtk_box_pack_start(GTK_BOX(button_row), quit_button, TRUE, TRUE, 0);
    gtk_box_pack_start(GTK_BOX(box), title, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box), detail, TRUE, TRUE, 0);
    gtk_box_pack_start(GTK_BOX(box), button_row, FALSE, FALSE, 0);
    gtk_container_add(GTK_CONTAINER(state->fallback_window), box);

    g_signal_connect(open_button, "clicked", G_CALLBACK(ls_tray_open_dashboard_button), state);
    g_signal_connect_swapped(quit_button, "clicked", G_CALLBACK(g_application_quit), state->application);

    gtk_widget_show_all(state->fallback_window);
}

static void ls_tray_backend_wait_done(GObject *source, GAsyncResult *result, gpointer user_data) {
    LSTrayApp *state = user_data;
    GSubprocess *process = G_SUBPROCESS(source);
    GError *error = NULL;

    if (!g_subprocess_wait_finish(process, result, &error)) {
        g_clear_error(&error);
        return;
    }

    if (state->intentionally_stopping || process != state->backend_process) {
        return;
    }

    g_clear_object(&state->backend_process);
    state->backend_owned = FALSE;
    state->status = LS_SERVICE_STATUS_FAILED;
    ls_tray_set_message(state, "The bundled LlamaSitter service stopped unexpectedly. Use Retry to start it again.");
    ls_tray_update_menu(state, NULL);
}

static gboolean ls_tray_start_backend(LSTrayApp *state) {
    GError *error = NULL;

    if (state->backend_process != NULL) {
        return TRUE;
    }

    state->backend_process = ls_launch_backend(&state->runtime_info, &error);
    if (state->backend_process == NULL) {
        state->status = LS_SERVICE_STATUS_FAILED;
        ls_tray_set_message(state, error != NULL ? error->message : "Unable to start the bundled LlamaSitter service.");
        g_clear_error(&error);
        ls_tray_update_menu(state, NULL);
        return FALSE;
    }

    state->backend_owned = TRUE;
    state->status = LS_SERVICE_STATUS_STARTING;
    ls_tray_set_message(state, "Launching the bundled local service.");
    g_subprocess_wait_async(state->backend_process, NULL, ls_tray_backend_wait_done, state);
    ls_tray_update_menu(state, NULL);
    return TRUE;
}

static void ls_tray_ensure_service(LSTrayApp *state) {
    if (ls_is_listen_addr_in_use(state->runtime_info.proxy_listen_addr) ||
        ls_is_listen_addr_in_use(state->runtime_info.ui_listen_addr)) {
        state->attached_to_external_service = TRUE;
        state->status = LS_SERVICE_STATUS_STARTING;
        ls_tray_set_message(state, "Connecting to the existing LlamaSitter service.");
        ls_tray_update_menu(state, NULL);
        return;
    }

    if (state->runtime_info.attach_only) {
        state->attached_to_external_service = TRUE;
        state->status = LS_SERVICE_STATUS_STARTING;
        ls_tray_set_message(state, "Waiting for an externally started LlamaSitter service.");
        ls_tray_update_menu(state, NULL);
        return;
    }

    state->attached_to_external_service = FALSE;
    ls_tray_start_backend(state);
}

static gboolean ls_tray_refresh(gpointer user_data) {
    LSTrayApp *state = user_data;
    LSDesktopOverview overview = {0};
    GError *error = NULL;

    if (ls_is_ready(state->runtime_info.ready_url, &error) &&
        ls_fetch_overview(state->runtime_info.ui_base_url, &overview, &error)) {
        state->status = LS_SERVICE_STATUS_READY;
        ls_tray_set_message(state, "");
        ls_tray_update_menu(state, &overview);
        ls_desktop_overview_clear(&overview);
        return G_SOURCE_CONTINUE;
    }

    g_clear_error(&error);
    if (state->status != LS_SERVICE_STATUS_FAILED) {
        state->status = LS_SERVICE_STATUS_STARTING;
        if (state->attached_to_external_service) {
            ls_tray_set_message(state, "Waiting for the local API to become ready.");
        } else if (state->backend_owned) {
            ls_tray_set_message(state, "Launching the bundled local service.");
        }
    }
    ls_tray_update_menu(state, NULL);
    ls_desktop_overview_clear(&overview);
    return G_SOURCE_CONTINUE;
}

static void ls_tray_open_dashboard(GtkMenuItem *menu_item, gpointer user_data) {
    LSTrayApp *state = user_data;
    GError *error = NULL;
    (void) menu_item;

    if (!ls_spawn_mode(state->self_executable, "dashboard", state->runtime_info.config_path, FALSE, &error)) {
        g_warning("Unable to open dashboard: %s", error != NULL ? error->message : "unknown error");
        g_clear_error(&error);
    }
}

static void ls_tray_retry(GtkMenuItem *menu_item, gpointer user_data) {
    LSTrayApp *state = user_data;
    GError *error = NULL;
    (void) menu_item;

    state->intentionally_stopping = FALSE;
    if (state->backend_process != NULL && state->backend_owned) {
        ls_stop_backend(state->backend_process, &error);
        g_clear_error(&error);
        g_clear_object(&state->backend_process);
        state->backend_owned = FALSE;
    }
    state->status = LS_SERVICE_STATUS_IDLE;
    ls_tray_set_message(state, "");
    ls_tray_ensure_service(state);
}

static void ls_tray_quit(GtkMenuItem *menu_item, gpointer user_data) {
    LSTrayApp *state = user_data;
    GError *error = NULL;
    (void) menu_item;

    state->intentionally_stopping = TRUE;
    if (state->backend_process != NULL && state->backend_owned) {
        ls_stop_backend(state->backend_process, &error);
        g_clear_error(&error);
    }
    g_application_quit(G_APPLICATION(state->application));
}

static GtkWidget *ls_tray_info_item(const gchar *label) {
    GtkWidget *item = gtk_menu_item_new_with_label(label);
    gtk_widget_set_sensitive(item, FALSE);
    return item;
}

static void ls_tray_build_menu(LSTrayApp *state) {
    GtkWidget *open_item = NULL;
    GtkWidget *separator = NULL;
    GtkWidget *quit_item = NULL;

    state->menu = gtk_menu_new();
    state->status_item = ls_tray_info_item("Status: Starting");
    state->updated_item = ls_tray_info_item("Updated: Waiting for the local API");
    state->requests_item = ls_tray_info_item("Requests: 0");
    state->tokens_item = ls_tray_info_item("Total Tokens: 0");
    state->duration_item = ls_tray_info_item("Average Duration: 0 ms");
    state->model_item = ls_tray_info_item("Top Model: No data yet");
    state->instance_item = ls_tray_info_item("Top Instance: No data yet");
    state->activity_title_item = ls_tray_info_item("Latest Activity: No activity yet");
    state->activity_detail_item = ls_tray_info_item("Requests and sessions will appear here once the proxy captures traffic.");

    open_item = gtk_menu_item_new_with_label("Open Dashboard");
    state->retry_item = gtk_menu_item_new_with_label("Retry");
    quit_item = gtk_menu_item_new_with_label("Quit LlamaSitter");

    g_signal_connect(open_item, "activate", G_CALLBACK(ls_tray_open_dashboard), state);
    g_signal_connect(state->retry_item, "activate", G_CALLBACK(ls_tray_retry), state);
    g_signal_connect(quit_item, "activate", G_CALLBACK(ls_tray_quit), state);

    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->status_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->updated_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->requests_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->tokens_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->duration_item);
    separator = gtk_separator_menu_item_new();
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), separator);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->model_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->instance_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->activity_title_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->activity_detail_item);
    separator = gtk_separator_menu_item_new();
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), separator);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), open_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), state->retry_item);
    gtk_menu_shell_append(GTK_MENU_SHELL(state->menu), quit_item);
    gtk_widget_show_all(state->menu);
}

static void ls_tray_activate(GApplication *application, gpointer user_data) {
    LSTrayApp *state = user_data;
    (void) application;

    if (state->refresh_source_id == 0) {
        ls_tray_ensure_service(state);
        state->refresh_source_id = g_timeout_add_seconds(5, ls_tray_refresh, state);
        ls_tray_refresh(state);
    }

    if (state->fallback_window != NULL) {
        gtk_window_present(GTK_WINDOW(state->fallback_window));
    }
}

static void ls_tray_startup(GApplication *application, gpointer user_data) {
    LSTrayApp *state = user_data;
    (void) application;

    ls_tray_build_menu(state);

#if defined(LS_HAVE_APPINDICATOR)
    if (ls_status_notifier_host_available()) {
        state->indicator = app_indicator_new("llamasitter-tray", "llamasitter", APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
        app_indicator_set_status(state->indicator, APP_INDICATOR_STATUS_ACTIVE);
        app_indicator_set_menu(state->indicator, GTK_MENU(state->menu));
        app_indicator_set_title(state->indicator, "LlamaSitter");
    } else {
        ls_tray_show_fallback_window(state);
    }
#else
    ls_tray_show_fallback_window(state);
#endif
}

static void ls_tray_shutdown(GApplication *application, gpointer user_data) {
    LSTrayApp *state = user_data;
    GError *error = NULL;
    (void) application;

    if (state->refresh_source_id != 0) {
        g_source_remove(state->refresh_source_id);
        state->refresh_source_id = 0;
    }

    if (state->backend_process != NULL && state->backend_owned) {
        ls_stop_backend(state->backend_process, &error);
        g_clear_error(&error);
    }

    g_clear_object(&state->backend_process);
    ls_runtime_info_clear(&state->runtime_info);
    g_clear_pointer(&state->self_executable, g_free);
    g_clear_pointer(&state->message, g_free);
}

int ls_tray_run(int argc, char **argv, const gchar *self_executable, const gchar *config_override, gboolean attach_only) {
    LSTrayApp state = {0};
    GError *error = NULL;
    gchar *cli_executable = NULL;
    gchar *application_id = NULL;
    int status = 1;

    state.self_executable = g_strdup(self_executable);
    cli_executable = ls_find_cli_executable(self_executable);
    if (!ls_runtime_info_load(cli_executable, config_override, attach_only, &state.runtime_info, &error)) {
        g_printerr("%s\n", error != NULL ? error->message : "Unable to resolve the desktop runtime.");
        g_clear_error(&error);
        g_clear_pointer(&cli_executable, g_free);
        g_clear_pointer(&state.self_executable, g_free);
        return 1;
    }
    g_clear_pointer(&cli_executable, g_free);

    application_id = ls_application_id_for_config("com.trevorashby.LlamaSitter.Tray", state.runtime_info.config_path);
    state.application = GTK_APPLICATION(gtk_application_new(application_id, G_APPLICATION_DEFAULT_FLAGS));
    g_free(application_id);
    state.status = LS_SERVICE_STATUS_IDLE;
    ls_tray_set_message(&state, "");

    g_signal_connect(state.application, "startup", G_CALLBACK(ls_tray_startup), &state);
    g_signal_connect(state.application, "activate", G_CALLBACK(ls_tray_activate), &state);
    g_signal_connect(state.application, "shutdown", G_CALLBACK(ls_tray_shutdown), &state);
    status = g_application_run(G_APPLICATION(state.application), argc, argv);
    g_object_unref(state.application);
    return status;
}
