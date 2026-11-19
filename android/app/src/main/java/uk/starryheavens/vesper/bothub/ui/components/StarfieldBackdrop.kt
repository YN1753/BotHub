package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

/**
 * Theme-adaptive flat backdrop without aurora glow or GPU overdraw.
 */
@Composable
fun StarfieldBackdrop(modifier: Modifier = Modifier) {
    val theme = BotHubThemeTokens
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(theme.bgBase)
    )
}
