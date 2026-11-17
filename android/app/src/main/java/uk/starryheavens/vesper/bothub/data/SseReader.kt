package uk.starryheavens.vesper.bothub.data

import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStream
import java.io.InputStreamReader

object SseReader {

    fun readJsonEvents(
        stream: InputStream,
        onEvent: (event: String, data: JSONObject) -> Unit
    ) {
        BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { reader ->
            var currentEvent = "message"
            var line: String?

            while (reader.readLine().also { line = it } != null) {
                val rawLine = line ?: continue
                if (rawLine.isBlank()) {
                    currentEvent = "message"
                    continue
                }
                val trimmed = rawLine.trim()
                if (trimmed.startsWith(":")) {
                    continue
                }
                when {
                    trimmed.startsWith("event:") -> {
                        currentEvent = trimmed.removePrefix("event:").trim().ifEmpty { "message" }
                    }
                    trimmed.startsWith("data:") -> {
                        val dataStr = trimmed.removePrefix("data:").trim()
                        if (dataStr.isNotEmpty()) {
                            val dataObj = runCatching { JSONObject(dataStr) }.getOrNull()
                            if (dataObj != null) {
                                onEvent(currentEvent, dataObj)
                            }
                        }
                    }
                }
            }
        }
    }
}
