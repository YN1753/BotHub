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
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AgentSessionsSheet(
    sessions: List<AgentSessionSummary>,
    currentSessionId: String,
    onSelectSession: (AgentSessionSummary) -> Unit,
    onNewSession: () -> Unit,
    onDismiss: () -> Unit
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        containerColor = Color(0xFF090E1D),
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        LazyColumn(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 6.dp),
            contentPadding = PaddingValues(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                        Text(
                            text = "跨端历史会话",
                            style = MaterialTheme.typography.titleLarge,
                            fontSize = 17.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE)
                        )
                        Text(
                            text = "共 ${sessions.size} 条自由会话 · 常驻精灵消息已实时云同步",
                            fontSize = 11.5.sp,
                            color = Color(0xFF94A3B8)
                        )
                    }
                    Surface(
                        onClick = onNewSession,
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0x24F2B880),
                        border = BorderStroke(1.dp, Color(0x66F2B880)),
                        modifier = Modifier.sizeIn(minHeight = 38.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 13.dp, vertical = 7.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                Icons.Default.Add,
                                contentDescription = null,
                                tint = Color(0xFFFFE2BE),
                                modifier = Modifier.size(15.dp)
                            )
                            Text(
                                text = "新对话",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFE2BE)
                            )
                        }
                    }
                }
            }

            if (sessions.isEmpty()) {
                item {
                    Surface(
                        shape = RoundedCornerShape(16.dp),
                        color = Color(0x990D1326),
                        border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 6.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 20.dp, vertical = 22.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp)
                        ) {
                            Text(
                                text = "当前为常驻精灵直连模式",
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFF8EE)
                            )
                            Text(
                                text = "每位常驻精灵的上下文自动持久保存；点击右上角「新对话」可开启全新临时会话",
                                fontSize = 12.sp,
                                lineHeight = 18.sp,
                                color = Color(0xFF94A3B8),
                                textAlign = TextAlign.Center
                            )
                        }
                    }
                }
            }

            items(sessions, key = { it.id }) { sess ->
                val active = sess.id == currentSessionId
                Surface(
                    onClick = { onSelectSession(sess) },
                    shape = RoundedCornerShape(16.dp),
                    color = if (active) Color(0x26F2B880) else Color(0xBF0C1224),
                    border = BorderStroke(
                        1.dp,
                        if (active) Color(0x80F2B880) else Color(0x20FFF8EE)
                    ),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Text(
                            text = sess.title,
                            fontSize = 14.5.sp,
                            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                            color = if (active) Color(0xFFFFE2BE) else Color(0xFFFFF8EE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = "${sess.persona} · ${sess.model} · ${sess.updatedAt.take(16)}",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = Color(0xFF94A3B8),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
            }
        }
    }
}
