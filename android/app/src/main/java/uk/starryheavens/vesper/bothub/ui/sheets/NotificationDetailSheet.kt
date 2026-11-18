package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationDetailSheet(
    item: NotificationItem,
    onCopyBody: (String) -> Unit,
    onDiagnoseWithAi: (NotificationItem) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val (badgeLabel, badgeColor) = formatModuleBadge(item.moduleId)

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        containerColor = Color(0xFF090E1D),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(modifier = Modifier.fillMaxWidth()) {
            // Header
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
                    Surface(
                        shape = RoundedCornerShape(6.dp),
                        color = badgeColor.copy(alpha = 0.15f),
                        border = BorderStroke(0.5.dp, badgeColor.copy(alpha = 0.42f))
                    ) {
                        Text(
                            text = badgeLabel,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                            color = badgeColor,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.5.dp)
                        )
                    }
                    Text(
                        text = item.createdAt,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = Color(0xFF94A3B8)
                    )
                }

                Text(
                    text = item.title,
                    style = MaterialTheme.typography.titleLarge,
                    fontSize = 18.sp,
                    lineHeight = 25.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFFFF8EE)
                )
            }

            // Scrollable Rich Markdown Body
            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false)
                    .padding(horizontal = 20.dp),
                contentPadding = PaddingValues(top = 8.dp, bottom = 14.dp)
            ) {
                item {
                    Surface(
                        shape = RoundedCornerShape(18.dp),
                        color = Color(0xCC0D1326),
                        border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        RichMarkdownText(
                            markdown = item.markdownBody,
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
            }

            // Sticky Bottom Action Bar (Always reachable without scrolling long reports)
            HorizontalDivider(thickness = 0.5.dp, color = Color(0x20FFF8EE))
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .padding(horizontal = 20.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                Surface(
                    onClick = { onCopyBody(item.markdownBody) },
                    shape = RoundedCornerShape(14.dp),
                    color = Color(0xFF11182E),
                    border = BorderStroke(1.dp, Color(0x29FFF8EE)),
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 46.dp)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally)
                    ) {
                        Icon(
                            Icons.Default.ContentCopy,
                            contentDescription = null,
                            tint = Color(0xFFCBD5E1),
                            modifier = Modifier.size(15.dp)
                        )
                        Text(
                            text = "复制原文",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = Color(0xFFE2E8F0)
                        )
                    }
                }

                Button(
                    onClick = { onDiagnoseWithAi(item) },
                    shape = RoundedCornerShape(14.dp),
                    modifier = Modifier
                        .weight(1.25f)
                        .sizeIn(minHeight = 46.dp)
                ) {
                    Text("交给 AI 深度解读", fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold)
                }
            }
        }
    }
}
