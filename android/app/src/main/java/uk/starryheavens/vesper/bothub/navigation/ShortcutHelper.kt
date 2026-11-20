package uk.starryheavens.vesper.bothub.navigation

import android.content.Context
import android.content.Intent
import androidx.core.app.Person
import androidx.core.content.pm.ShortcutInfoCompat
import androidx.core.content.pm.ShortcutManagerCompat
import androidx.core.graphics.drawable.IconCompat
import uk.starryheavens.vesper.bothub.QuickCaptureActivity
import uk.starryheavens.vesper.bothub.R

object ShortcutHelper {
    private const val CATEGORY_QUICK_STASH = "uk.starryheavens.vesper.bothub.category.QUICK_STASH"
    private const val SHORTCUT_ID_DIRECT_SHARE = "bothub_direct_share_stash"
    private const val SHORTCUT_ID_CLIPBOARD = "bothub_capture_clipboard"

    fun publishShortcuts(context: Context) {
        val appContext = context.applicationContext
        runCatching {
            val icon = IconCompat.createWithResource(appContext, R.mipmap.ic_launcher)

            val clipboardShortcut = ShortcutInfoCompat.Builder(appContext, SHORTCUT_ID_CLIPBOARD)
                .setShortLabel("收藏剪贴板")
                .setLongLabel("读取剪贴板并收藏到 BotHub")
                .setIcon(icon)
                .setIntent(AppIntents.createClipboardCaptureIntent(appContext))
                .setRank(0)
                .build()

            val directShareIntent = Intent(appContext, QuickCaptureActivity::class.java).apply {
                action = Intent.ACTION_SEND
                type = "text/plain"
            }
            val person = Person.Builder()
                .setName("收藏到 BotHub")
                .setKey(SHORTCUT_ID_DIRECT_SHARE)
                .build()

            val directShareShortcut = ShortcutInfoCompat.Builder(appContext, SHORTCUT_ID_DIRECT_SHARE)
                .setShortLabel("收藏到 BotHub")
                .setLongLabel("一键收入 BotHub 收藏匣")
                .setIcon(icon)
                .setIntent(directShareIntent)
                .setCategories(setOf(CATEGORY_QUICK_STASH))
                .setPerson(person)
                .setLongLived(true)
                .setRank(0)
                .build()

            ShortcutManagerCompat.pushDynamicShortcut(appContext, clipboardShortcut)
            ShortcutManagerCompat.pushDynamicShortcut(appContext, directShareShortcut)
        }
    }
}
