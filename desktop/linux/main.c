#include "common.h"

#include <glib.h>
#include <string.h>

int ls_dashboard_run(int argc, char **argv, const gchar *self_executable, const gchar *config_override, gboolean attach_only);
int ls_tray_run(int argc, char **argv, const gchar *self_executable, const gchar *config_override, gboolean attach_only);

static const gchar *ls_mode_from_args(int argc, char **argv) {
    gint i = 1;
    while (i < argc) {
        if (g_str_has_prefix(argv[i], "--mode=")) {
            return argv[i] + strlen("--mode=");
        }
        if (g_strcmp0(argv[i], "--mode") == 0 && i + 1 < argc) {
            return argv[i + 1];
        }
        i += 1;
    }
    return "dashboard";
}

static gchar *ls_config_override_from_args(int argc, char **argv) {
    gint i = 1;
    while (i < argc) {
        if (g_strcmp0(argv[i], "--config") == 0 && i + 1 < argc) {
            return g_strdup(argv[i + 1]);
        }
        if (g_str_has_prefix(argv[i], "--config=")) {
            return g_strdup(argv[i] + strlen("--config="));
        }
        i += 1;
    }
    if (g_getenv("LLAMASITTER_DESKTOP_CONFIG") != NULL) {
        return g_strdup(g_getenv("LLAMASITTER_DESKTOP_CONFIG"));
    }
    return NULL;
}

static gboolean ls_attach_only_from_args(int argc, char **argv) {
    gint i = 1;
    const gchar *env_value = g_getenv("LLAMASITTER_DESKTOP_ATTACH_ONLY");

    if (env_value != NULL &&
        (g_ascii_strcasecmp(env_value, "1") == 0 ||
         g_ascii_strcasecmp(env_value, "true") == 0 ||
         g_ascii_strcasecmp(env_value, "yes") == 0)) {
        return TRUE;
    }

    while (i < argc) {
        if (g_strcmp0(argv[i], "--attach-only") == 0) {
            return TRUE;
        }
        i += 1;
    }
    return FALSE;
}

int main(int argc, char **argv) {
    const gchar *mode = NULL;
    gchar *self_executable = NULL;
    gchar *config_override = NULL;
    gboolean attach_only = FALSE;
    int status = 1;

    mode = ls_mode_from_args(argc, argv);
    self_executable = ls_resolve_self_executable(argv[0]);
    config_override = ls_config_override_from_args(argc, argv);
    attach_only = ls_attach_only_from_args(argc, argv);

    if (g_strcmp0(mode, "tray") == 0) {
        status = ls_tray_run(argc, argv, self_executable, config_override, attach_only);
    } else {
        status = ls_dashboard_run(argc, argv, self_executable, config_override, attach_only);
    }

    g_free(config_override);
    g_free(self_executable);
    return status;
}
