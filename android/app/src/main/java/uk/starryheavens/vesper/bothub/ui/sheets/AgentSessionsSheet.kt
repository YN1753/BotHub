package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary

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
        shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
        containerColor = MaterialTheme.colorScheme.surface
    ) {
        LazyColumn(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 8.dp),
            contentPadding = PaddingValues(bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column {
                        Text(
                            text = "跨端历史会话",
                            style = MaterialTheme.typography.titleLarge,
                            fontSize = 18.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE)
                        )
                        Text(
                            text = "共 ${sessions.size} 条云端同步对话",
                            fontSize = 12.sp,
                            color = Color(0xFF94A3B8)
                        )
                    }
                    Button(
                        onClick = onNewSession,
                        shape = RoundedCornerShape(12.dp),
                        modifier = Modifier.sizeIn(minHeight = 42.dp)
                    ) {
                        Text("+ 新对话", fontWeight = FontWeight.SemiBold)
                    }
                }
            }
            if (sessions.isEmpty()) {
                item {
                    Text(
                        text = "暂无历史会话记录",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(vertical = 16.dp)
                    )
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
