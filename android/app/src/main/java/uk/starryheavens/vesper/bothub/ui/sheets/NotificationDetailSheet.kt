package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpecularHairline
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationDetailSheet(
    item: NotificationItem,
    onCopyBody: (String) -> Unit,
    onConvertToTodo: (NotificationItem) -> Unit = {},
    onSaveToNote: (NotificationItem) -> Unit = {},
    onDelete: (NotificationItem) -> Unit = {},
    onDiagnoseWithAi: (NotificationItem) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val (badgeLabel, badgeColor) = formatModuleBadge(item.moduleId)

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp, vertical = 6.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(6.dp))
                            .background(badgeColor.copy(alpha = 0.15f))
                            .border(0.5.dp, badgeColor.copy(alpha = 0.42f), RoundedCornerShape(6.dp))
                            .padding(horizontal = 8.dp, vertical = 2.5.dp)
                    ) {
                        Text(
                            text = badgeLabel,
                            style = TextStyle(
                                fontSize = 10.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = badgeColor,
                                platformStyle = NoFontPaddingStyle
                            )
                        )
                    }
                    Text(
                        text = item.createdAt,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                }

                Text(
                    text = item.title,
                    fontSize = 18.sp,
                    lineHeight = 25.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary
                )
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false)
                    .padding(horizontal = 20.dp),
                contentPadding = PaddingValues(top = 8.dp, bottom = 14.dp)
            ) {
                item {
                    SpecularGlassCard(
                        cornerRadius = 18.dp,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        RichMarkdownText(
                            markdown = item.markdownBody,
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
            }

            SpecularHairline()
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .padding(horizontal = 20.dp, vertical = 10.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    TactileSecondaryButton(
                        text = "转为待办",
                        accent = true,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                        onClick = { onConvertToTodo(item) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = "存为手记",
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                        onClick = { onSaveToNote(item) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                        onClick = { onDelete(item) },
                        modifier = Modifier.weight(0.78f)
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    TactileSecondaryButton(
                        text = "复制原文",
                        leadingIcon = Icons.Default.ContentCopy,
                        minHeight = 44.dp,
                        onClick = { onCopyBody(item.markdownBody) },
                        modifier = Modifier.weight(0.95f)
                    )

                    TactilePrimaryButton(
                        text = "找 TA 追问 / 处理",
                        leadingIcon = Icons.Default.AutoAwesome,
                        minHeight = 44.dp,
                        onClick = { onDiagnoseWithAi(item) },
                        modifier = Modifier.weight(1.3f)
                    )
                }
            }
        }
    }
}
