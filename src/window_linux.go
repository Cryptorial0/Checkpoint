//go:build linux && !nowebview

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <stdlib.h>
#include <stdio.h>
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static void on_destroy(GtkWidget* widget, gpointer data) {
    gtk_main_quit();
}

static int run_gtk_webview(const char* title, const char* url, int width, int height, const void* icon_data, int icon_len) {
    // 1. Wayland requirement: Set program name and application name before gtk_init_check
    g_set_prgname("checkpoint");
    g_set_application_name("Checkpoint");

    if (!gtk_init_check(NULL, NULL)) {
        return 0;
    }

    // 2. Add AppDir paths to GTK icon search path if running inside an AppImage
    const char* appdir = g_getenv("APPDIR");
    if (appdir) {
        GtkIconTheme *theme = gtk_icon_theme_get_default();
        if (theme) {
            gtk_icon_theme_append_search_path(theme, appdir);
            char icons_path[1024];
            snprintf(icons_path, sizeof(icons_path), "%s/usr/share/icons/hicolor", appdir);
            gtk_icon_theme_append_search_path(theme, icons_path);
        }
    }

    // 3. Set default icon name for window decorations and Wayland compositors
    gtk_window_set_default_icon_name("checkpoint");

    GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_title(GTK_WINDOW(window), title);
    gtk_window_set_default_size(GTK_WINDOW(window), width, height);
    gtk_window_set_position(GTK_WINDOW(window), GTK_WIN_POS_CENTER);
    gtk_window_set_icon_name(GTK_WINDOW(window), "checkpoint");

    if (icon_data && icon_len > 0) {
        GdkPixbufLoader *loader = gdk_pixbuf_loader_new();
        if (loader) {
            if (gdk_pixbuf_loader_write(loader, (const guint8*)icon_data, (gsize)icon_len, NULL)) {
                gdk_pixbuf_loader_close(loader, NULL);
                GdkPixbuf *pix = gdk_pixbuf_loader_get_pixbuf(loader);
                if (pix) {
                    gtk_window_set_default_icon(pix);
                    gtk_window_set_icon(GTK_WINDOW(window), pix);
                }
            } else {
                gdk_pixbuf_loader_close(loader, NULL);
            }
            g_object_unref(loader);
        }
    }

    g_signal_connect(window, "destroy", G_CALLBACK(on_destroy), NULL);

    GtkWidget *webview = webkit_web_view_new();

    WebKitSettings *settings = webkit_web_view_get_settings(WEBKIT_WEB_VIEW(webview));
    webkit_settings_set_enable_developer_extras(settings, TRUE);
    webkit_settings_set_enable_webgl(settings, TRUE);

    gtk_container_add(GTK_CONTAINER(window), webview);

    webkit_web_view_load_uri(WEBKIT_WEB_VIEW(webview), url);

    gtk_widget_show_all(window);
    gtk_main();
    return 1;
}
*/
import "C"
import (
	"os"
	"path/filepath"
	"unsafe"
)

func ensureLinuxDesktopIcon() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps")
	iconPath := filepath.Join(iconDir, "checkpoint.png")
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		iconBytes, _ := webFiles.ReadFile("web/icon.png")
		if len(iconBytes) > 0 {
			_ = os.MkdirAll(iconDir, 0755)
			_ = os.WriteFile(iconPath, iconBytes, 0644)
		}
	}
}

func runEmbeddedAppWindow(url string) bool {
	ensureLinuxDesktopIcon()

	cTitle := C.CString("Checkpoint")
	defer C.free(unsafe.Pointer(cTitle))
	cUrl := C.CString(url)
	defer C.free(unsafe.Pointer(cUrl))

	iconBytes, _ := webFiles.ReadFile("web/icon.png")
	var iconPtr unsafe.Pointer
	var iconLen C.int
	if len(iconBytes) > 0 {
		iconPtr = unsafe.Pointer(&iconBytes[0])
		iconLen = C.int(len(iconBytes))
	}

	Log("Launching standalone native WebKitGTK desktop window...")
	res := C.run_gtk_webview(cTitle, cUrl, C.int(980), C.int(820), iconPtr, iconLen)
	return res == 1
}
