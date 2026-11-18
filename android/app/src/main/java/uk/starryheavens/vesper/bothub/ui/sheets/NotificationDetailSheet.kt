package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationDetailSheet(
    item: NotificationItem,
    onCopyBody: (String) -> Unit,
    onDiagnoseWithAi: (NotificationItem) -> Unit,
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
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleLarge,
                        fontSize = 19.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = "${item.moduleId} · ${item.createdAt}",
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = Color(0xFF94A3B8)
                    )
                }
            }
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
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    OutlinedButton(
                        onClick = { onCopyBody(item.markdownBody) },
                        shape = RoundedCornerShape(14.dp),
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Icon(
                                Icons.Default.ContentCopy,
                                contentDescription = null,
                                modifier = Modifier.size(16.dp)
                            )
                            Text("复制原文")
                        }
                    }
                    Button(
                        onClick = { onDiagnoseWithAi(item) },
                        shape = RoundedCornerShape(14.dp),
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("交给 AI 诊断", fontWeight = FontWeight.SemiBold)
                    }
                }
            }
        }
    }
}
