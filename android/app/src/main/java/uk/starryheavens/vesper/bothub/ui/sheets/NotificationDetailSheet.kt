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
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
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
    isRerolling: Boolean = false,
    isTodoSaved: Boolean = false,
    isNoteSaved: Boolean = false,
    onCopyBody: (String) -> Unit,
    onConvertToTodo: (NotificationItem) -> Unit = {},
    onSaveToNote: (NotificationItem) -> Unit = {},
    onReroll: (NotificationItem, String) -> Unit = { _, _ -> },
    onDelete: (NotificationItem) -> Unit = {},
    onDiagnoseWithAi: (NotificationItem) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val (badgeLabel, badgeColor) = formatModuleBadge(item.moduleId)
    var showRerollInput by remember(item.id) { mutableStateOf(false) }
    var rerollFeedback by remember(item.id) { mutableStateOf("") }

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
                if (showRerollInput) {
                    ObsidianGlassInput(
                        value = rerollFeedback,
                        onValueChange = { rerollFeedback = it },
                        label = "告诉 TA 哪里不满意或想看什么（可留空直接换方向）",
                        placeholder = "例如：少推纯理论，多推具体实操工具",
                        singleLine = false,
                        maxLines = 2
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        TactileSecondaryButton(
                            text = "取消",
                            cornerRadius = 10.dp,
                            minHeight = 36.dp,
                            enabled = !isRerolling,
                            onClick = {
                                showRerollInput = false
                                rerollFeedback = ""
                            },
                            modifier = Modifier.weight(0.65f)
                        )
                        TactilePrimaryButton(
                            text = if (isRerolling) "正在重新生成..." else "立即原位重推并记入偏好",
                            leadingIcon = Icons.Default.Refresh,
                            minHeight = 36.dp,
                            enabled = !isRerolling,
                            onClick = {
                                onReroll(item, rerollFeedback.trim())
                                showRerollInput = false
                                rerollFeedback = ""
                            },
                            modifier = Modifier.weight(1.6f)
                        )
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    TactileSecondaryButton(
                        text = if (isTodoSaved) "已建待办" else "转为待办",
                        accent = !isTodoSaved,
                        enabled = !isTodoSaved,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 5.dp),
                        onClick = { onConvertToTodo(item) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = if (isNoteSaved) "已存笔记" else "存为笔记",
                        enabled = !isNoteSaved,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 5.dp),
                        onClick = { onSaveToNote(item) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = if (isRerolling) "重推中..." else "换一条",
                        leadingIcon = Icons.Default.Refresh,
                        enabled = !isRerolling,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 5.dp),
                        onClick = { showRerollInput = !showRerollInput },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        cornerRadius = 10.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 5.dp),
                        onClick = { onDelete(item) },
                        modifier = Modifier.weight(0.72f)
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
                        text = "找 TA 追问",
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
