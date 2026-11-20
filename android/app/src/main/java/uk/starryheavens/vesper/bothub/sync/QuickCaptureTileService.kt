package uk.starryheavens.vesper.bothub.sync

import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import uk.starryheavens.vesper.bothub.navigation.AppIntents

class QuickCaptureTileService : TileService() {

    override fun onStartListening() {
        super.onStartListening()
        qsTile?.apply {
            state = Tile.STATE_INACTIVE
            label = "BotHub 收藏"
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                subtitle = "读取剪贴板"
            }
            updateTile()
        }
    }

    override fun onClick() {
        super.onClick()
        runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                startActivityAndCollapse(AppIntents.createClipboardCapturePendingIntent(this, 9202))
            } else {
                @Suppress("DEPRECATION")
                startActivityAndCollapse(AppIntents.createClipboardCaptureIntent(this))
            }
        }
    }
}
