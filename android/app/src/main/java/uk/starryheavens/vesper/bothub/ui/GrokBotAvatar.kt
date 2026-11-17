package uk.starryheavens.vesper.bothub.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotItem
import kotlin.math.PI
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin

data class BotPaletteUi(
    val ring: Color,
    val shellHi: Color,
    val shellMid: Color,
    val shellDeep: Color,
    val aurora: Color,
    val eyeInk: Color,
    val blush: Color,
    val isDarkBody: Boolean = false
)

private val GROKBOT_PALETTES = mapOf(
    "gold" to BotPaletteUi(
        ring = Color(0xFFF2B880),
        shellHi = Color(0xFFFFFFFF),
        shellMid = Color(0xFFF6EDE2),
        shellDeep = Color(0xFFD8C2AA),
        aurora = Color(0xFFFFB87A),
        eyeInk = Color(0xFF131826),
        blush = Color(0xFFFF8570)
    ),
    "indigo" to BotPaletteUi(
        ring = Color(0xFFA5B4FC),
        shellHi = Color(0xFFFFFFFF),
        shellMid = Color(0xFFE6EEFF),
        shellDeep = Color(0xFFB2C5F2),
        aurora = Color(0xFF7DD3FC),
        eyeInk = Color(0xFF10172B),
        blush = Color(0xFFFB7185)
    ),
    "emerald" to BotPaletteUi(
        ring = Color(0xFF34D399),
        shellHi = Color(0xFFFFFFFF),
        shellMid = Color(0xFFE2FAEF),
        shellDeep = Color(0xFF9EDBC3),
        aurora = Color(0xFF34D399),
        eyeInk = Color(0xFF0F201D),
        blush = Color(0xFFFB923C)
    ),
    "amber" to BotPaletteUi(
        ring = Color(0xFFFBBF24),
        shellHi = Color(0xFFFFFEFB),
        shellMid = Color(0xFFFFEFD6),
        shellDeep = Color(0xFFEFC48E),
        aurora = Color(0xFFFB923C),
        eyeInk = Color(0xFF1F1512),
        blush = Color(0xFFF43F5E)
    ),
    "violet" to BotPaletteUi(
        ring = Color(0xFFD8B4FE),
        shellHi = Color(0xFFFFFFFF),
        shellMid = Color(0xFFF2E7FF),
        shellDeep = Color(0xFFC9B1EE),
        aurora = Color(0xFFF472B6),
        eyeInk = Color(0xFF1A1228),
        blush = Color(0xFFFB7185)
    ),
    "obsidian" to BotPaletteUi(
        ring = Color(0xFF38BDF8),
        shellHi = Color(0xFF374669),
        shellMid = Color(0xFF182036),
        shellDeep = Color(0xFF090D1A),
        aurora = Color(0xFF38BDF8),
        eyeInk = Color(0xFFF0F9FF),
        blush = Color(0xFFF472B6),
        isDarkBody = true
    )
)

fun parseBotAvatarStyle(raw: String?): Pair<String, BotPaletteUi> {
    val clean = (raw ?: "cyan").trim().lowercase()
    val (shape, colorKey) = if (clean.contains(":")) {
        val parts = clean.split(":", limit = 2)
        val s = parts[0].takeIf { it in setOf("butler", "sentinel", "radar", "critic", "swarm", "visor") } ?: "butler"
        val c = parts[1].takeIf { GROKBOT_PALETTES.containsKey(it) } ?: "gold"
        s to c
    } else {
        when (clean) {
            "indigo" -> "sentinel" to "indigo"
            "emerald" -> "radar" to "emerald"
            "amber" -> "critic" to "amber"
            "violet" -> "swarm" to "violet"
            else -> "butler" to "gold"
        }
    }
    return shape to (GROKBOT_PALETTES[colorKey] ?: GROKBOT_PALETTES.getValue("gold"))
}

private fun buildMonolithicPath(shape: String): Path = Path().apply {
    when (shape) {
        "sentinel" -> {
            moveTo(28f, 13.2f)
            cubicTo(33f, 13.2f, 36.4f, 7f, 40.2f, 7f)
            cubicTo(44.6f, 7f, 48.6f, 18.6f, 48.6f, 31.8f)
            cubicTo(48.6f, 42.8f, 40.6f, 48.6f, 28f, 48.6f)
            cubicTo(15.4f, 48.6f, 7.4f, 42.8f, 7.4f, 31.8f)
            cubicTo(7.4f, 18.6f, 11.4f, 7f, 15.8f, 7f)
            cubicTo(19.6f, 7f, 23f, 13.2f, 28f, 13.2f)
        }
        "radar" -> {
            moveTo(28f, 5.4f)
            cubicTo(33.6f, 5.4f, 48.6f, 18.8f, 48.6f, 32.4f)
            cubicTo(48.6f, 43f, 40.4f, 48.6f, 28f, 48.6f)
            cubicTo(15.6f, 48.6f, 7.4f, 43f, 7.4f, 32.4f)
            cubicTo(7.4f, 18.8f, 22.4f, 5.4f, 28f, 5.4f)
        }
        "critic" -> {
            moveTo(28f, 14.4f)
            cubicTo(35.6f, 14.4f, 40.8f, 5.8f, 45f, 5.8f)
            cubicTo(48.4f, 5.8f, 46.2f, 16.4f, 47.8f, 26.8f)
            cubicTo(49.2f, 35.9f, 43.4f, 48.6f, 28f, 48.6f)
            cubicTo(12.6f, 48.6f, 6.8f, 35.9f, 8.2f, 26.8f)
            cubicTo(9.8f, 16.4f, 7.6f, 5.8f, 11f, 5.8f)
            cubicTo(15.2f, 5.8f, 20.4f, 14.4f, 28f, 14.4f)
        }
        "swarm" -> {
            moveTo(28f, 6.4f)
            cubicTo(34.4f, 6.4f, 36.8f, 11.6f, 41.2f, 13f)
            cubicTo(46.8f, 14.8f, 50.4f, 20.6f, 50.4f, 28.4f)
            cubicTo(50.4f, 36.2f, 46.2f, 42.4f, 40.4f, 45.4f)
            cubicTo(36.4f, 47.5f, 32.6f, 48.6f, 28f, 48.6f)
            cubicTo(23.4f, 48.6f, 19.6f, 47.5f, 15.6f, 45.4f)
            cubicTo(9.8f, 42.4f, 5.6f, 36.2f, 5.6f, 28.4f)
            cubicTo(5.6f, 20.6f, 9.2f, 14.8f, 14.8f, 13f)
            cubicTo(19.2f, 11.6f, 21.6f, 6.4f, 28f, 6.4f)
        }
        "visor" -> {
            moveTo(28f, 8.2f)
            cubicTo(43.2f, 8.2f, 47.8f, 12.8f, 47.8f, 28f)
            cubicTo(47.8f, 43.2f, 43.2f, 47.8f, 28f, 47.8f)
            cubicTo(12.8f, 47.8f, 8.2f, 43.2f, 8.2f, 28f)
            cubicTo(8.2f, 12.8f, 12.8f, 8.2f, 28f, 8.2f)
        }
        else -> {
            moveTo(28f, 7.6f)
            cubicTo(39.8f, 7.6f, 48.8f, 18.2f, 48.8f, 31.8f)
            cubicTo(48.8f, 42.8f, 40.8f, 48.6f, 28f, 48.6f)
            cubicTo(15.2f, 48.6f, 7.2f, 42.8f, 7.2f, 31.8f)
            cubicTo(7.2f, 18.2f, 16.2f, 7.6f, 28f, 7.6f)
        }
    }
    close()
}

private fun buildHappyCrescentPath(x0: Float): Path = Path().apply {
    moveTo(x0, 28.8f)
    cubicTo(x0 + 0.8f, 23.4f, x0 + 8f, 23.4f, x0 + 8.8f, 28.8f)
    cubicTo(x0 + 9f, 30.1f, x0 + 7.4f, 30.8f, x0 + 6.6f, 29.7f)
    cubicTo(x0 + 5.5f, 27.6f, x0 + 3.3f, 27.6f, x0 + 2.2f, 29.7f)
    cubicTo(x0 + 1.4f, 30.8f, x0 - 0.2f, 30.1f, x0, 28.8f)
    close()
}

private fun buildFourPointSparkle(cx: Float, cy: Float, r: Float): Path = Path().apply {
    moveTo(cx, cy - r)
    quadraticBezierTo(cx, cy, cx + r, cy)
    quadraticBezierTo(cx, cy, cx, cy + r)
    quadraticBezierTo(cx, cy, cx - r, cy)
    quadraticBezierTo(cx, cy, cx, cy - r)
    close()
}

fun resolveGroupMembers(groupBot: BotItem, allBots: List<BotItem>): List<BotItem> =
    groupBot.resolveGroupMembers(allBots)

/**
 * Native Jetpack Compose Monolithic Soft-Body GrokBot Avatar.
 * Shares exact 1:1 Bezier geometry, subsurface aurora shading, 6 archetype idle personalities,
 * and 4 tap-triggered physical stunts (hop / flip / dance / shy) with the Web Vue component.
 */
@Composable
fun GrokBotAvatar(
    avatarStyle: String = "cyan",
    state: String = "idle",
    size: Dp = 38.dp,
    interactive: Boolean = false,
    isListening: Boolean = false,
    seedKey: String = avatarStyle,
    modifier: Modifier = Modifier
) {
    val haptics = LocalHapticFeedback.current
    val scope = rememberCoroutineScope()
    val (shape, palette) = remember(avatarStyle) { parseBotAvatarStyle(avatarStyle) }
    val bodyPath = remember(shape) { buildMonolithicPath(shape) }
    val happyLeft = remember { buildHappyCrescentPath(16.8f) }
    val happyRight = remember { buildHappyCrescentPath(30.4f) }

    var stuntIndex by remember { mutableIntStateOf(0) }
    var activeStunt by remember { mutableStateOf<String?>(null) }
    val stuntProgress = remember { Animatable(0f) }

    val seedPhase = remember(seedKey) {
        var h = 0
        for (ch in seedKey) h = (h * 31 + ch.code) % 97
        (h % 37) * 0.17f
    }

    val infinite = rememberInfiniteTransition(label = "grokbot_loop")
    val masterPhase by infinite.animateFloat(
        initialValue = 0f,
        targetValue = (2.0 * PI).toFloat(),
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 5600, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "master_phase"
    )

    val p = masterPhase + seedPhase
    val floatWave = sin(p * 1.5f)

    // 6 distinct archetype personality offsets + state choreographies
    var rigDx = 0f
    var rigDy = -floatWave * 1.8f
    var rigRot = 0f
    var rigScaleX = 1f - floatWave * 0.026f
    var rigScaleY = 1f + floatWave * 0.026f

    val effectiveState = if (activeStunt != null && activeStunt != "shy") "done" else state

    when (effectiveState) {
        "thinking" -> {
            rigRot = sin(p * 2f) * 5.2f
            rigDy -= 1.0f
        }
        "working" -> {
            val fast = sin(p * 4f)
            rigDy = -abs(fast) * 3.0f
            rigScaleX = 1f + fast * 0.04f
            rigScaleY = 1f - fast * 0.04f
            rigRot = cos(p * 4f) * 3.5f
        }
        "done" -> {
            val hop = abs(sin(p * 2.5f))
            rigDy -= hop * 2.6f
            rigScaleX = 1f - hop * 0.03f
            rigScaleY = 1f + hop * 0.04f
        }
        "blocked" -> {
            rigDx = sin(p * 8f) * 1.4f
        }
        else -> {
            // Idle personality actions per archetype
            val burst = ((sin(p) + 1f) * 0.5f)
            if (burst > 0.62f) {
                val t = (burst - 0.62f) / 0.38f
                val pulse = sin(t * PI.toFloat())
                when (shape) {
                    "butler" -> {
                        rigRot = sin(t * 2f * PI.toFloat()) * 6f
                        rigDy -= pulse * 2.4f
                    }
                    "sentinel" -> {
                        rigScaleY += pulse * 0.055f
                        rigScaleX -= pulse * 0.035f
                        rigRot = sin(t * 2f * PI.toFloat()) * 7f
                    }
                    "radar" -> {
                        rigDx = sin(t * 2f * PI.toFloat()) * 2.2f
                        rigRot = rigDx * 2.1f
                    }
                    "critic" -> {
                        val hop2 = abs(sin(t * 2f * PI.toFloat()))
                        rigDy -= hop2 * 3.6f
                        rigRot = sin(t * 2f * PI.toFloat()) * 5f
                    }
                    "swarm" -> {
                        rigScaleX += pulse * 0.065f
                        rigScaleY += pulse * 0.035f
                        rigRot = sin(t * 2f * PI.toFloat()) * 3.2f
                    }
                    "visor" -> {
                        rigScaleX += sin(t * 2f * PI.toFloat()) * 0.07f
                        rigScaleY -= sin(t * 2f * PI.toFloat()) * 0.07f
                    }
                }
            }
        }
    }

    if (isListening && state == "idle" && activeStunt == null) {
        rigDy += 1.0f
        rigScaleX *= 1.045f
        rigScaleY *= 0.965f
    }

    // Apply 4 interactive tap stunts (hop, flip, dance, shy)
    val sp = stuntProgress.value
    if (activeStunt != null) {
        when (activeStunt) {
            "hop" -> {
                val arc = sin(sp * PI.toFloat())
                rigDy -= arc * 7.5f
                rigScaleX = if (sp < 0.2f) 1.16f else 1f - arc * 0.08f
                rigScaleY = if (sp < 0.2f) 0.84f else 1f + arc * 0.1f
            }
            "flip" -> {
                val arc = sin(sp * PI.toFloat())
                rigDy -= arc * 8.0f
                rigRot += sp * 360f
            }
            "dance" -> {
                val wave = sin(sp * 4f * PI.toFloat())
                rigDx += wave * 2.8f
                rigDy -= abs(wave) * 3.2f
                rigRot += wave * 11f
            }
            "shy" -> {
                val hold = sin(sp * PI.toFloat())
                rigDy += hold * 4.2f
                rigScaleX = 1f + hold * 0.2f
                rigScaleY = 1f - hold * 0.22f
                rigRot += sin(sp * 3f * PI.toFloat()) * 2.5f * hold
            }
        }
    }

    val clickMod = if (interactive) {
        Modifier.clickable(
            interactionSource = remember { MutableInteractionSource() },
            indication = null,
            role = Role.Button
        ) {
            haptics.performHapticFeedback(HapticFeedbackType.LongPress)
            val stunts = listOf("hop", "flip", "dance", "shy")
            val chosen = stunts[stuntIndex % stunts.size]
            stuntIndex += 1
            scope.launch {
                activeStunt = chosen
                stuntProgress.snapTo(0f)
                stuntProgress.animateTo(
                    targetValue = 1f,
                    animationSpec = tween(
                        durationMillis = if (chosen == "flip" || chosen == "dance") 740 else 620,
                        easing = FastOutSlowInEasing
                    )
                )
                delay(60L)
                activeStunt = null
                stuntProgress.snapTo(0f)
            }
        }
    } else {
        Modifier
    }

    val eyeOffsetY = when (shape) {
        "critic" -> 1.8f
        "radar" -> 1.2f
        "visor" -> -0.6f
        else -> 0.4f
    }

    val auroraColor = when {
        state == "blocked" && activeStunt == null -> Color(0xFFFB7185)
        effectiveState == "done" -> Color(0xFF34D399)
        state == "thinking" -> Color(0xFFFBBF24)
        else -> palette.aurora
    }

    val eyeColor = if (state == "blocked" && activeStunt == null) {
        if (palette.isDarkBody) Color(0xFFFDA4AF) else Color(0xFFBE123C)
    } else {
        palette.eyeInk
    }

    // Natural blink & eye glance
    val blinkNorm = (p % (2f * PI.toFloat())) / (2f * PI.toFloat())
    val blinkScaleY = if ((blinkNorm in 0.34f..0.38f) || (blinkNorm in 0.84f..0.875f)) 0.1f else 1f
    val gazeDx = when {
        state == "working" -> sin(p * 4.5f) * 2.4f
        state == "thinking" -> sin(p * 2f) * 1.8f
        else -> if (blinkNorm in 0.44f..0.62f) 2.0f else if (blinkNorm in 0.68f..0.80f) -1.8f else 0f
    }
    val gazeDy = if (state == "thinking") -2.2f else 0f

    Canvas(
        modifier = modifier
            .size(size)
            .then(clickMod)
            .graphicsLayer {
                val pxScale = this.size.minDimension / 56f
                translationX = rigDx * pxScale
                translationY = rigDy * pxScale
                rotationZ = rigRot
                scaleX = rigScaleX
                scaleY = rigScaleY
            }
    ) {
        val unit = this.size.minDimension / 56f
        withTransform({
            scale(scaleX = unit, scaleY = unit, pivot = Offset.Zero)
        }) {
            // Soft outer aura + ground shadow
            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(auroraColor.copy(alpha = 0.24f), Color.Transparent),
                    center = Offset(28f, 29f),
                    radius = 27f
                ),
                radius = 27f,
                center = Offset(28f, 29f)
            )
            drawOval(
                color = Color(0x8803060F),
                topLeft = Offset(16f, 50.2f),
                size = Size(24f, 3.6f)
            )

            // Seamless Monolithic Clipped Body
            clipPath(bodyPath) {
                drawPath(
                    path = bodyPath,
                    brush = Brush.radialGradient(
                        0.0f to palette.shellHi,
                        0.56f to palette.shellMid,
                        1.0f to palette.shellDeep,
                        center = Offset(19f, 14f),
                        radius = 35f
                    )
                )

                val auroraDx = sin(p) * 5.5f
                val auroraDy = cos(p) * 4.5f
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            auroraColor.copy(alpha = if (palette.isDarkBody) 0.52f else 0.42f),
                            auroraColor.copy(alpha = 0.12f),
                            Color.Transparent
                        ),
                        center = Offset(35f + auroraDx, 37f + auroraDy),
                        radius = 21f
                    ),
                    radius = 21f,
                    center = Offset(35f + auroraDx, 37f + auroraDy)
                )

                // Top dome airbrushed lustre
                drawOval(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            Color.White.copy(alpha = if (palette.isDarkBody) 0.30f else 0.80f),
                            Color.Transparent
                        ),
                        center = Offset(21.5f, 15.5f),
                        radius = 13f
                    ),
                    topLeft = Offset(8.5f, 7f),
                    size = Size(26f, 17f)
                )

                // Face rig: Cheek blush + GrokBot Capsule Eyes
                translate(left = gazeDx, top = eyeOffsetY + gazeDy) {
                    val blushAlpha = if (effectiveState == "done" || activeStunt == "shy") 0.78f else 0.56f
                    for (bx in listOf(15.4f, 40.6f)) {
                        drawOval(
                            brush = Brush.radialGradient(
                                colors = listOf(palette.blush.copy(alpha = blushAlpha), Color.Transparent),
                                center = Offset(bx, 33.2f),
                                radius = 6.8f
                            ),
                            topLeft = Offset(bx - 6.8f, 28.8f),
                            size = Size(13.6f, 8.8f)
                        )
                    }

                    when {
                        state == "blocked" && activeStunt == null -> {
                            rotate(degrees = 14f, pivot = Offset(21.2f, 27.5f)) {
                                drawRoundRect(
                                    color = eyeColor,
                                    topLeft = Offset(18.6f, 22.4f),
                                    size = Size(5.2f, 10.2f),
                                    cornerRadius = CornerRadius(2.6f, 2.6f)
                                )
                            }
                            rotate(degrees = -14f, pivot = Offset(34.8f, 27.5f)) {
                                drawRoundRect(
                                    color = eyeColor,
                                    topLeft = Offset(32.2f, 22.4f),
                                    size = Size(5.2f, 10.2f),
                                    cornerRadius = CornerRadius(2.6f, 2.6f)
                                )
                            }
                        }
                        activeStunt == "shy" -> {
                            drawRoundRect(
                                color = eyeColor,
                                topLeft = Offset(17.2f, 27.2f),
                                size = Size(7.2f, 3.6f),
                                cornerRadius = CornerRadius(1.8f, 1.8f)
                            )
                            drawRoundRect(
                                color = eyeColor,
                                topLeft = Offset(31.6f, 27.2f),
                                size = Size(7.2f, 3.6f),
                                cornerRadius = CornerRadius(1.8f, 1.8f)
                            )
                        }
                        effectiveState == "done" -> {
                            drawPath(path = happyLeft, color = eyeColor)
                            drawPath(path = happyRight, color = eyeColor)
                        }
                        else -> {
                            if (palette.isDarkBody || state == "working" || state == "thinking") {
                                drawRoundRect(
                                    color = auroraColor.copy(alpha = 0.28f),
                                    topLeft = Offset(17.8f, 21f),
                                    size = Size(7f, 13.2f),
                                    cornerRadius = CornerRadius(3.5f, 3.5f)
                                )
                                drawRoundRect(
                                    color = auroraColor.copy(alpha = 0.28f),
                                    topLeft = Offset(31.2f, 21f),
                                    size = Size(7f, 13.2f),
                                    cornerRadius = CornerRadius(3.5f, 3.5f)
                                )
                            }

                            val rightSquint = if (state == "thinking") 0.76f else 1f
                            val leftH = 11.2f * blinkScaleY
                            val rightH = 11.2f * blinkScaleY * rightSquint

                            drawRoundRect(
                                color = eyeColor,
                                topLeft = Offset(18.8f, 27.6f - leftH * 0.5f),
                                size = Size(5f, leftH),
                                cornerRadius = CornerRadius(2.5f, 2.5f)
                            )
                            drawRoundRect(
                                color = eyeColor,
                                topLeft = Offset(32.2f, 27.6f - rightH * 0.5f),
                                size = Size(5f, rightH),
                                cornerRadius = CornerRadius(2.5f, 2.5f)
                            )
                            if (blinkScaleY > 0.5f) {
                                drawCircle(
                                    color = Color.White.copy(alpha = 0.9f),
                                    radius = 1.15f,
                                    center = Offset(20.5f, 24.7f)
                                )
                                drawCircle(
                                    color = Color.White.copy(alpha = 0.9f),
                                    radius = 1.15f,
                                    center = Offset(33.9f, 25.1f)
                                )
                            }
                        }
                    }
                }
            }

            // Subtle continuous rim sheen
            drawPath(
                path = bodyPath,
                brush = Brush.linearGradient(
                    colors = listOf(
                        Color.White.copy(alpha = if (palette.isDarkBody) 0.42f else 0.72f),
                        auroraColor.copy(alpha = 0.45f)
                    ),
                    start = Offset(12f, 8f),
                    end = Offset(44f, 48f)
                ),
                style = Stroke(width = 0.9f)
            )

            // Geometric Vector Emotes (Zero Emojis)
            if (effectiveState == "done" || activeStunt == "hop" || activeStunt == "flip" || activeStunt == "dance") {
                val twinkle = 0.75f + 0.35f * abs(sin(p * 3f))
                drawPath(buildFourPointSparkle(48.5f, 12.2f, 3.6f * twinkle), color = palette.ring)
                drawPath(buildFourPointSparkle(7.5f, 16f, 2.5f * twinkle), color = Color.White.copy(alpha = 0.9f))
            } else if (state == "thinking") {
                val bob = sin(p * 3f) * 1.2f
                drawCircle(color = palette.shellHi.copy(alpha = 0.75f), radius = 1.4f, center = Offset(44.5f, 14.5f + bob))
                drawCircle(color = palette.shellHi.copy(alpha = 0.86f), radius = 2.1f, center = Offset(48.5f, 10.2f - bob))
                drawCircle(color = auroraColor.copy(alpha = 0.92f), radius = 2.9f, center = Offset(52.2f, 5.4f + bob))
            } else if (state == "working") {
                drawArc(
                    color = auroraColor.copy(alpha = 0.78f),
                    startAngle = 195f,
                    sweepAngle = 52f,
                    useCenter = false,
                    topLeft = Offset(4f, 6f),
                    size = Size(48f, 44f),
                    style = Stroke(width = 1.8f, cap = StrokeCap.Round)
                )
                drawArc(
                    color = palette.ring.copy(alpha = 0.78f),
                    startAngle = 15f,
                    sweepAngle = 52f,
                    useCenter = false,
                    topLeft = Offset(4f, 6f),
                    size = Size(48f, 44f),
                    style = Stroke(width = 1.8f, cap = StrokeCap.Round)
                )
            } else if (state == "blocked") {
                drawRoundRect(
                    color = Color(0xFFFB7185),
                    topLeft = Offset(26.4f, 0.5f),
                    size = Size(3.2f, 6.2f),
                    cornerRadius = CornerRadius(1.6f, 1.6f)
                )
                drawCircle(color = Color(0xFFFB7185), radius = 1.5f, center = Offset(28f, 9.2f))
            }
        }
    }
}

private data class ClusterPoseUi(
    val txFrac: Float,
    val tyFrac: Float,
    val scale: Float,
    val rotDeg: Float,
    val z: Float
)

private fun resolveClusterPose(slot: Int, total: Int): ClusterPoseUi {
    if (total == 2) {
        return if (slot == 0) {
            ClusterPoseUi(0.15f, 0.11f, 0.77f, 6f, 30f)
        } else {
            ClusterPoseUi(-0.17f, -0.10f, 0.69f, -13f, 10f)
        }
    }
    if (total == 3) {
        return when (slot) {
            0 -> ClusterPoseUi(0f, 0.14f, 0.73f, 0f, 30f)
            1 -> ClusterPoseUi(-0.24f, -0.10f, 0.63f, -15f, 15f)
            else -> ClusterPoseUi(0.24f, -0.10f, 0.63f, 15f, 14f)
        }
    }
    return when (slot) {
        0 -> ClusterPoseUi(0f, 0.15f, 0.71f, 0f, 30f)
        1 -> ClusterPoseUi(-0.25f, -0.01f, 0.61f, -16f, 20f)
        2 -> ClusterPoseUi(0.25f, -0.01f, 0.61f, 16f, 19f)
        else -> ClusterPoseUi(0f, -0.18f, 0.58f, 3f, 10f)
    }
}

/**
 * Unboxed Snuggled Mochi Group Cluster (叠叠乐贴贴团) for Jetpack Compose.
 * Smoothly promotes the speaking or tapped member to Front-Center C-Position.
 */
@Composable
fun GrokBotGroupCluster(
    members: List<BotItem>,
    fallbackStyle: String = "swarm:indigo",
    state: String = "idle",
    activeSpeakerId: String = "",
    size: Dp = 40.dp,
    interactive: Boolean = false,
    modifier: Modifier = Modifier
) {
    val display = remember(members) { members.take(4) }
    if (display.size <= 1) {
        GrokBotAvatar(
            avatarStyle = display.firstOrNull()?.avatarStyle ?: fallbackStyle,
            state = state,
            size = size,
            interactive = interactive,
            modifier = modifier
        )
        return
    }

    val haptics = LocalHapticFeedback.current
    var pinnedLeadId by remember { mutableStateOf("") }

    val leadId = when {
        activeSpeakerId.isNotEmpty() && display.any { it.id == activeSpeakerId } -> activeSpeakerId
        display.any { it.state == "working" || it.state == "thinking" } ->
            display.first { it.state == "working" || it.state == "thinking" }.id
        pinnedLeadId.isNotEmpty() && display.any { it.id == pinnedLeadId } -> pinnedLeadId
        else -> display.first().id
    }

    val slotMap = remember(display, leadId) {
        val map = mutableMapOf<String, Int>()
        var back = 1
        for (m in display) {
            if (m.id == leadId) map[m.id] = 0 else map[m.id] = back++
        }
        map
    }

    val hasActiveSpeaker = remember(display, activeSpeakerId) {
        (activeSpeakerId.isNotEmpty() && display.any { it.id == activeSpeakerId }) ||
            display.any { it.state == "working" || it.state == "thinking" }
    }

    Box(
        modifier = modifier.size(width = size * 1.16f, height = size),
        contentAlignment = Alignment.Center
    ) {
        display.forEach { m ->
            key(m.id) {
                val slot = slotMap[m.id] ?: 0
                val pose = resolveClusterPose(slot, display.size)
                val speakingBoost = slot == 0 && hasActiveSpeaker
                val targetTy = if (speakingBoost) pose.tyFrac - 0.04f else pose.tyFrac
                val targetSc = if (speakingBoost) pose.scale * 1.08f else pose.scale

                val animTx by animateFloatAsState(
                    targetValue = pose.txFrac,
                    animationSpec = spring(dampingRatio = 0.62f, stiffness = Spring.StiffnessMediumLow),
                    label = "c_tx"
                )
                val animTy by animateFloatAsState(
                    targetValue = targetTy,
                    animationSpec = spring(dampingRatio = 0.62f, stiffness = Spring.StiffnessMediumLow),
                    label = "c_ty"
                )
                val animSc by animateFloatAsState(
                    targetValue = targetSc,
                    animationSpec = spring(dampingRatio = 0.62f, stiffness = Spring.StiffnessMediumLow),
                    label = "c_sc"
                )
                val animRot by animateFloatAsState(
                    targetValue = pose.rotDeg,
                    animationSpec = spring(dampingRatio = 0.68f, stiffness = Spring.StiffnessMediumLow),
                    label = "c_rot"
                )

                val memberState = when {
                    activeSpeakerId.isNotEmpty() && m.id == activeSpeakerId -> "working"
                    m.state != "idle" -> m.state
                    slot == 0 && state != "idle" -> state
                    else -> "idle"
                }

                val tapPromote = if (interactive && slot != 0) {
                    Modifier.clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button
                    ) {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        pinnedLeadId = m.id
                    }
                } else {
                    Modifier
                }

                Box(
                    modifier = Modifier
                        .size(size)
                        .zIndex(if (speakingBoost) 40f else pose.z)
                        .graphicsLayer {
                            translationX = animTx * this.size.width * 1.12f
                            translationY = animTy * this.size.height
                            scaleX = animSc
                            scaleY = animSc
                            rotationZ = animRot
                        }
                        .then(tapPromote),
                    contentAlignment = Alignment.Center
                ) {
                    GrokBotAvatar(
                        avatarStyle = m.avatarStyle,
                        state = memberState,
                        size = size,
                        interactive = interactive && slot == 0,
                        seedKey = m.id
                    )
                }
            }
        }
    }
}
