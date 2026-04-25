#include "common.h"

#include <errno.h>
#include <fcntl.h>
#include <gio/gdesktopappinfo.h>
#include <json-glib/json-glib.h>
#include <libsoup/soup.h>
#include <signal.h>
#include <string.h>
#include <unistd.h>

static gboolean ls_json_has_string(JsonObject *object, const gchar *member) {
    return json_object_has_member(object, member) &&
           JSON_NODE_HOLDS_VALUE(json_object_get_member(object, member));
}

static gchar *ls_json_dup_string(JsonObject *object, const gchar *member) {
    if (!ls_json_has_string(object, member)) {
        return NULL;
    }
    return g_strdup(json_object_get_string_member(object, member));
}

static gint64 ls_json_get_int64(JsonObject *object, const gchar *member) {
    if (!json_object_has_member(object, member)) {
        return 0;
    }
    return json_object_get_int_member(object, member);
}

static gdouble ls_json_get_double(JsonObject *object, const gchar *member) {
    if (!json_object_has_member(object, member)) {
        return 0.0;
    }
    return json_object_get_double_member(object, member);
}

static gboolean ls_json_get_bool(JsonObject *object, const gchar *member) {
    if (!json_object_has_member(object, member)) {
        return FALSE;
    }
    return json_object_get_boolean_member(object, member);
}

static gboolean ls_subprocess_capture_json(GSubprocess *process,
                                           gchar **stdout_text,
                                           GError **error) {
    gchar *stdout_buf = NULL;
    gchar *stderr_buf = NULL;
    gboolean ok = g_subprocess_communicate_utf8(process, NULL, NULL, &stdout_buf, &stderr_buf, error);
    if (!ok) {
        g_free(stdout_buf);
        g_free(stderr_buf);
        return FALSE;
    }

    if (!g_subprocess_get_successful(process)) {
        g_set_error(error,
                    G_IO_ERROR,
                    G_IO_ERROR_FAILED,
                    "command failed: %s",
                    stderr_buf != NULL && *stderr_buf != '\0' ? stderr_buf : "unknown error");
        g_free(stdout_buf);
        g_free(stderr_buf);
        return FALSE;
    }

    *stdout_text = stdout_buf;
    g_free(stderr_buf);
    return TRUE;
}

static gboolean ls_http_ok(const gchar *url, gchar **body_out, guint *status_out, GError **error) {
    SoupSession *session = soup_session_new();
    SoupMessage *message = soup_message_new("GET", url);
    GBytes *bytes = NULL;
    guint status_code = 0;
    gboolean ok = FALSE;

    if (message == NULL) {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "unable to create request for %s", url);
        g_object_unref(session);
        return FALSE;
    }

    bytes = soup_session_send_and_read(session, message, NULL, error);
    status_code = soup_message_get_status(message);
    if (status_out != NULL) {
        *status_out = status_code;
    }

    if (bytes == NULL) {
        g_object_unref(message);
        g_object_unref(session);
        return FALSE;
    }

    if (SOUP_STATUS_IS_SUCCESSFUL(status_code)) {
        gsize size = 0;
        const gchar *data = g_bytes_get_data(bytes, &size);
        if (body_out != NULL) {
            *body_out = g_strndup(data, size);
        }
        ok = TRUE;
    } else {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "request to %s failed with HTTP %u", url, status_code);
    }

    g_bytes_unref(bytes);
    g_object_unref(message);
    g_object_unref(session);
    return ok;
}

static gboolean ls_dbus_name_has_owner(const gchar *name) {
    GDBusConnection *connection = NULL;
    GVariant *reply = NULL;
    gboolean has_owner = FALSE;
    GError *error = NULL;

    connection = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &error);
    if (connection == NULL) {
        g_clear_error(&error);
        return FALSE;
    }

    reply = g_dbus_connection_call_sync(connection,
                                        "org.freedesktop.DBus",
                                        "/org/freedesktop/DBus",
                                        "org.freedesktop.DBus",
                                        "NameHasOwner",
                                        g_variant_new("(s)", name),
                                        G_VARIANT_TYPE("(b)"),
                                        G_DBUS_CALL_FLAGS_NONE,
                                        1000,
                                        NULL,
                                        &error);
    if (reply != NULL) {
        g_variant_get(reply, "(b)", &has_owner);
        g_variant_unref(reply);
    } else {
        g_clear_error(&error);
    }

    g_object_unref(connection);
    return has_owner;
}

void ls_runtime_info_clear(LSRuntimeInfo *runtime_info) {
    if (runtime_info == NULL) {
        return;
    }

    g_clear_pointer(&runtime_info->platform, g_free);
    g_clear_pointer(&runtime_info->application_support_dir, g_free);
    g_clear_pointer(&runtime_info->config_dir, g_free);
    g_clear_pointer(&runtime_info->state_dir, g_free);
    g_clear_pointer(&runtime_info->config_path, g_free);
    g_clear_pointer(&runtime_info->db_path, g_free);
    g_clear_pointer(&runtime_info->logs_path, g_free);
    g_clear_pointer(&runtime_info->app_log_path, g_free);
    g_clear_pointer(&runtime_info->backend_log_path, g_free);
    g_clear_pointer(&runtime_info->autostart_path, g_free);
    g_clear_pointer(&runtime_info->proxy_listen_addr, g_free);
    g_clear_pointer(&runtime_info->ui_listen_addr, g_free);
    g_clear_pointer(&runtime_info->ui_base_url, g_free);
    g_clear_pointer(&runtime_info->ready_url, g_free);
    g_clear_pointer(&runtime_info->backend_executable, g_free);
    runtime_info->attach_only = FALSE;
}

void ls_desktop_overview_clear(LSDesktopOverview *overview) {
    if (overview == NULL) {
        return;
    }

    g_clear_pointer(&overview->last_refresh_at, g_free);
    g_clear_pointer(&overview->top_model, g_free);
    g_clear_pointer(&overview->top_client_instance, g_free);
    g_clear_pointer(&overview->activity_title, g_free);
    g_clear_pointer(&overview->activity_detail, g_free);
    overview->request_count = 0;
    overview->total_tokens = 0;
    overview->average_request_duration_ms = 0.0;
}

gchar *ls_resolve_self_executable(const gchar *argv0) {
    GError *error = NULL;
    gchar *path = g_file_read_link("/proc/self/exe", &error);
    if (path != NULL) {
        return path;
    }
    g_clear_error(&error);

    if (argv0 != NULL && g_path_is_absolute(argv0)) {
        return g_strdup(argv0);
    }

    if (argv0 != NULL && *argv0 != '\0') {
        return g_find_program_in_path(argv0);
    }

    return NULL;
}

gchar *ls_find_cli_executable(const gchar *self_executable) {
    const gchar *override = g_getenv("LLAMASITTER_CLI_PATH");
    if (override != NULL && *override != '\0') {
        return g_strdup(override);
    }

    if (self_executable != NULL) {
        gchar *dir = g_path_get_dirname(self_executable);
        gchar *sibling = g_build_filename(dir, "llamasitter", NULL);
        if (g_file_test(sibling, G_FILE_TEST_IS_EXECUTABLE)) {
            g_free(dir);
            return sibling;
        }
        g_free(sibling);
        g_free(dir);
    }

    return g_find_program_in_path("llamasitter");
}

gchar *ls_application_id_for_config(const gchar *base_id, const gchar *config_path) {
    const gchar *key = config_path != NULL && *config_path != '\0' ? config_path : "default";
    gchar *digest = g_compute_checksum_for_string(G_CHECKSUM_SHA256, key, -1);
    gchar *application_id = g_strdup_printf("%s.%.*s", base_id, 12, digest);
    g_free(digest);
    return application_id;
}

gboolean ls_runtime_info_load(const gchar *cli_executable,
                              const gchar *config_override,
                              gboolean attach_only,
                              LSRuntimeInfo *runtime_info,
                              GError **error) {
    GSubprocessLauncher *launcher = NULL;
    GSubprocess *process = NULL;
    gchar *stdout_text = NULL;
    JsonParser *parser = NULL;
    JsonNode *root = NULL;
    JsonObject *object = NULL;
    const gchar *argv[8];
    gint argc = 0;

    if (cli_executable == NULL || *cli_executable == '\0') {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_NOT_FOUND, "unable to locate the llamasitter CLI executable");
        return FALSE;
    }

    launcher = g_subprocess_launcher_new(G_SUBPROCESS_FLAGS_STDOUT_PIPE | G_SUBPROCESS_FLAGS_STDERR_PIPE);
    if (attach_only) {
        g_subprocess_launcher_setenv(launcher, "LLAMASITTER_DESKTOP_ATTACH_ONLY", "1", TRUE);
    } else {
        g_subprocess_launcher_unsetenv(launcher, "LLAMASITTER_DESKTOP_ATTACH_ONLY");
    }

    argv[argc++] = cli_executable;
    if (config_override != NULL && *config_override != '\0') {
        argv[argc++] = "--config";
        argv[argc++] = config_override;
    }
    argv[argc++] = "desktop";
    argv[argc++] = "runtime";
    argv[argc++] = "--output";
    argv[argc++] = "json";
    argv[argc] = NULL;

    process = g_subprocess_launcher_spawnv(launcher, argv, error);
    g_object_unref(launcher);
    if (process == NULL) {
        return FALSE;
    }

    if (!ls_subprocess_capture_json(process, &stdout_text, error)) {
        g_object_unref(process);
        return FALSE;
    }
    g_object_unref(process);

    parser = json_parser_new();
    if (!json_parser_load_from_data(parser, stdout_text, -1, error)) {
        g_object_unref(parser);
        g_free(stdout_text);
        return FALSE;
    }
    g_free(stdout_text);

    root = json_parser_get_root(parser);
    if (root == NULL || !JSON_NODE_HOLDS_OBJECT(root)) {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "desktop runtime payload was not a JSON object");
        g_object_unref(parser);
        return FALSE;
    }

    object = json_node_get_object(root);
    ls_runtime_info_clear(runtime_info);
    runtime_info->platform = ls_json_dup_string(object, "platform");
    runtime_info->application_support_dir = ls_json_dup_string(object, "application_support_dir");
    runtime_info->config_dir = ls_json_dup_string(object, "config_dir");
    runtime_info->state_dir = ls_json_dup_string(object, "state_dir");
    runtime_info->config_path = ls_json_dup_string(object, "config_path");
    runtime_info->db_path = ls_json_dup_string(object, "db_path");
    runtime_info->logs_path = ls_json_dup_string(object, "logs_path");
    runtime_info->app_log_path = ls_json_dup_string(object, "app_log_path");
    runtime_info->backend_log_path = ls_json_dup_string(object, "backend_log_path");
    runtime_info->autostart_path = ls_json_dup_string(object, "autostart_path");
    runtime_info->proxy_listen_addr = ls_json_dup_string(object, "proxy_listen_addr");
    runtime_info->ui_listen_addr = ls_json_dup_string(object, "ui_listen_addr");
    runtime_info->ui_base_url = ls_json_dup_string(object, "ui_base_url");
    runtime_info->ready_url = ls_json_dup_string(object, "ready_url");
    runtime_info->backend_executable = ls_json_dup_string(object, "backend_executable");
    runtime_info->attach_only = ls_json_get_bool(object, "attach_only");

    g_object_unref(parser);

    if (runtime_info->config_path == NULL || runtime_info->ui_base_url == NULL || runtime_info->ready_url == NULL) {
        ls_runtime_info_clear(runtime_info);
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "desktop runtime payload was missing required fields");
        return FALSE;
    }

    return TRUE;
}

gboolean ls_is_ready(const gchar *ready_url, GError **error) {
    guint status_code = 0;
    gboolean ok = ls_http_ok(ready_url, NULL, &status_code, error);
    return ok && status_code == SOUP_STATUS_OK;
}

gboolean ls_wait_until_ready(const gchar *ready_url,
                             gint timeout_ms,
                             gint poll_interval_ms,
                             GCancellable *cancellable,
                             GError **error) {
    gint64 deadline = g_get_monotonic_time() + ((gint64) timeout_ms * 1000);

    while (g_get_monotonic_time() < deadline) {
        GError *ready_error = NULL;
        if (ls_is_ready(ready_url, &ready_error)) {
            return TRUE;
        }
        g_clear_error(&ready_error);

        if (cancellable != NULL && g_cancellable_is_cancelled(cancellable)) {
            g_set_error_literal(error, G_IO_ERROR, G_IO_ERROR_CANCELLED, "cancelled");
            return FALSE;
        }

        g_usleep((gulong) poll_interval_ms * 1000);
    }

    g_set_error(error, G_IO_ERROR, G_IO_ERROR_TIMED_OUT, "service did not become ready within %d ms", timeout_ms);
    return FALSE;
}

gboolean ls_fetch_overview(const gchar *base_url,
                           LSDesktopOverview *overview,
                           GError **error) {
    gchar *url = NULL;
    gchar *body = NULL;
    JsonParser *parser = NULL;
    JsonNode *root = NULL;
    JsonObject *object = NULL;

    url = g_strconcat(base_url, "/api/desktop/overview", NULL);
    if (!ls_http_ok(url, &body, NULL, error)) {
        g_free(url);
        return FALSE;
    }
    g_free(url);

    parser = json_parser_new();
    if (!json_parser_load_from_data(parser, body, -1, error)) {
        g_object_unref(parser);
        g_free(body);
        return FALSE;
    }
    g_free(body);

    root = json_parser_get_root(parser);
    if (root == NULL || !JSON_NODE_HOLDS_OBJECT(root)) {
        g_set_error_literal(error, G_IO_ERROR, G_IO_ERROR_FAILED, "desktop overview payload was not a JSON object");
        g_object_unref(parser);
        return FALSE;
    }

    object = json_node_get_object(root);
    ls_desktop_overview_clear(overview);
    overview->last_refresh_at = ls_json_dup_string(object, "last_refresh_at");
    overview->request_count = ls_json_get_int64(object, "request_count");
    overview->total_tokens = ls_json_get_int64(object, "total_tokens");
    overview->average_request_duration_ms = ls_json_get_double(object, "average_request_duration_ms");
    overview->top_model = ls_json_dup_string(object, "top_model");
    overview->top_client_instance = ls_json_dup_string(object, "top_client_instance");
    overview->activity_title = ls_json_dup_string(object, "activity_title");
    overview->activity_detail = ls_json_dup_string(object, "activity_detail");

    g_object_unref(parser);
    return TRUE;
}

gboolean ls_is_listen_addr_in_use(const gchar *listen_addr) {
    GSocketClient *client = NULL;
    GSocketConnectable *address = NULL;
    GSocketConnection *connection = NULL;
    GError *error = NULL;
    gboolean in_use = FALSE;

    address = g_network_address_parse(listen_addr, 0, &error);
    if (address == NULL) {
        g_clear_error(&error);
        return FALSE;
    }

    client = g_socket_client_new();
    g_socket_client_set_timeout(client, 1);
    connection = g_socket_client_connect(client, address, NULL, &error);
    if (connection != NULL) {
        in_use = TRUE;
        g_object_unref(connection);
    } else {
        g_clear_error(&error);
    }

    g_object_unref(client);
    g_object_unref(address);
    return in_use;
}

gboolean ls_status_notifier_host_available(void) {
    return ls_dbus_name_has_owner("org.kde.StatusNotifierWatcher") ||
           ls_dbus_name_has_owner("org.freedesktop.StatusNotifierWatcher");
}

GSubprocess *ls_launch_backend(const LSRuntimeInfo *runtime_info, GError **error) {
    GSubprocessLauncher *launcher = NULL;
    GSubprocess *process = NULL;
    const gchar *argv[5];
    gint log_fd = -1;
    gint stderr_fd = -1;
    gchar *log_dir = NULL;

    if (runtime_info == NULL || runtime_info->backend_executable == NULL || runtime_info->config_path == NULL) {
        g_set_error_literal(error, G_IO_ERROR, G_IO_ERROR_INVALID_ARGUMENT, "desktop runtime is incomplete");
        return NULL;
    }

    if (runtime_info->backend_log_path != NULL) {
        log_dir = g_path_get_dirname(runtime_info->backend_log_path);
        if (g_mkdir_with_parents(log_dir, 0755) != 0) {
            g_free(log_dir);
            g_set_error(error, G_IO_ERROR, g_io_error_from_errno(errno), "unable to create backend log directory");
            return NULL;
        }
        g_free(log_dir);

        log_fd = open(runtime_info->backend_log_path, O_CREAT | O_WRONLY | O_APPEND, 0644);
        if (log_fd < 0) {
            g_set_error(error,
                        G_IO_ERROR,
                        g_io_error_from_errno(errno),
                        "unable to open backend log at %s",
                        runtime_info->backend_log_path);
            return NULL;
        }
        stderr_fd = dup(log_fd);
    }

    launcher = g_subprocess_launcher_new(G_SUBPROCESS_FLAGS_NONE);
    g_subprocess_launcher_setenv(launcher, "LLAMASITTER_DESKTOP_MANAGED", "1", TRUE);
    if (log_fd >= 0) {
        g_subprocess_launcher_take_stdout_fd(launcher, log_fd);
        g_subprocess_launcher_take_stderr_fd(launcher, stderr_fd >= 0 ? stderr_fd : dup(log_fd));
    }

    argv[0] = runtime_info->backend_executable;
    argv[1] = "serve";
    argv[2] = "--config";
    argv[3] = runtime_info->config_path;
    argv[4] = NULL;

    process = g_subprocess_launcher_spawnv(launcher, argv, error);
    g_object_unref(launcher);
    return process;
}

gboolean ls_stop_backend(GSubprocess *process, GError **error) {
    if (process == NULL) {
        return TRUE;
    }

    if (g_subprocess_get_if_exited(process) || g_subprocess_get_if_signaled(process)) {
        return TRUE;
    }

    g_subprocess_send_signal(process, SIGTERM);
    if (!g_subprocess_wait(process, NULL, error)) {
        return FALSE;
    }
    return TRUE;
}

gboolean ls_spawn_mode(const gchar *self_executable,
                       const gchar *mode,
                       const gchar *config_path,
                       gboolean attach_only,
                       GError **error) {
    GPtrArray *argv = NULL;
    gboolean ok = FALSE;
    gchar **argvv = NULL;
    GSubprocessLauncher *launcher = NULL;
    GSubprocess *process = NULL;

    if (self_executable == NULL || *self_executable == '\0') {
        g_set_error_literal(error, G_IO_ERROR, G_IO_ERROR_NOT_FOUND, "desktop executable path is unavailable");
        return FALSE;
    }

    argv = g_ptr_array_new_with_free_func(g_free);
    g_ptr_array_add(argv, g_strdup(self_executable));
    g_ptr_array_add(argv, g_strdup_printf("--mode=%s", mode));
    if (config_path != NULL && *config_path != '\0') {
        g_ptr_array_add(argv, g_strdup("--config"));
        g_ptr_array_add(argv, g_strdup(config_path));
    }
    if (attach_only) {
        g_ptr_array_add(argv, g_strdup("--attach-only"));
    }
    g_ptr_array_add(argv, NULL);
    argvv = (gchar **) g_ptr_array_free(argv, FALSE);

    launcher = g_subprocess_launcher_new(G_SUBPROCESS_FLAGS_NONE);
    process = g_subprocess_launcher_spawnv(launcher, (const gchar * const *) argvv, error);
    g_object_unref(launcher);
    if (process != NULL) {
        ok = TRUE;
        g_object_unref(process);
    }

    g_strfreev(argvv);
    return ok;
}

gboolean ls_open_uri(const gchar *uri, GError **error) {
    return g_app_info_launch_default_for_uri(uri, NULL, error);
}

const gchar *ls_status_label(LSServiceStatus status) {
    switch (status) {
        case LS_SERVICE_STATUS_STARTING:
            return "Starting";
        case LS_SERVICE_STATUS_READY:
            return "Ready";
        case LS_SERVICE_STATUS_FAILED:
            return "Error";
        case LS_SERVICE_STATUS_IDLE:
        default:
            return "Stopped";
    }
}
