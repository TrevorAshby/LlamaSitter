#ifndef LLAMASITTER_LINUX_COMMON_H
#define LLAMASITTER_LINUX_COMMON_H

#include <gio/gio.h>
#include <glib.h>

typedef enum {
    LS_SERVICE_STATUS_IDLE = 0,
    LS_SERVICE_STATUS_STARTING,
    LS_SERVICE_STATUS_READY,
    LS_SERVICE_STATUS_FAILED,
} LSServiceStatus;

typedef struct {
    gchar *platform;
    gchar *application_support_dir;
    gchar *config_dir;
    gchar *state_dir;
    gchar *config_path;
    gchar *db_path;
    gchar *logs_path;
    gchar *app_log_path;
    gchar *backend_log_path;
    gchar *autostart_path;
    gchar *proxy_listen_addr;
    gchar *ui_listen_addr;
    gchar *ui_base_url;
    gchar *ready_url;
    gchar *backend_executable;
    gboolean attach_only;
} LSRuntimeInfo;

typedef struct {
    gchar *last_refresh_at;
    gint64 request_count;
    gint64 total_tokens;
    gdouble average_request_duration_ms;
    gchar *top_model;
    gchar *top_client_instance;
    gchar *activity_title;
    gchar *activity_detail;
} LSDesktopOverview;

void ls_runtime_info_clear(LSRuntimeInfo *runtime_info);
void ls_desktop_overview_clear(LSDesktopOverview *overview);

gchar *ls_resolve_self_executable(const gchar *argv0);
gchar *ls_find_cli_executable(const gchar *self_executable);
gchar *ls_application_id_for_config(const gchar *base_id, const gchar *config_path);

gboolean ls_runtime_info_load(const gchar *cli_executable,
                              const gchar *config_override,
                              gboolean attach_only,
                              LSRuntimeInfo *runtime_info,
                              GError **error);

gboolean ls_is_ready(const gchar *ready_url, GError **error);
gboolean ls_wait_until_ready(const gchar *ready_url,
                             gint timeout_ms,
                             gint poll_interval_ms,
                             GCancellable *cancellable,
                             GError **error);

gboolean ls_fetch_overview(const gchar *base_url,
                           LSDesktopOverview *overview,
                           GError **error);

gboolean ls_is_listen_addr_in_use(const gchar *listen_addr);
gboolean ls_status_notifier_host_available(void);

GSubprocess *ls_launch_backend(const LSRuntimeInfo *runtime_info, GError **error);
gboolean ls_stop_backend(GSubprocess *process, GError **error);

gboolean ls_spawn_mode(const gchar *self_executable,
                       const gchar *mode,
                       const gchar *config_path,
                       gboolean attach_only,
                       GError **error);

gboolean ls_open_uri(const gchar *uri, GError **error);
const gchar *ls_status_label(LSServiceStatus status);

#endif
