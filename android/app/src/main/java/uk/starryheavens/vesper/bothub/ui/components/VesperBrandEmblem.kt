package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.clipRect
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * Unified Vesper Evening-Star Spirit ("长庚星环糯团") squircle brand emblem,
 * matching Web VesperBrandIcon.vue, favicon.svg, and Android Adaptive Launcher Icon 1:1.
 */
@Composable
fun VesperBrandEmblem(
    size: Dp = 28.dp,
    modifier: Modifier = Modifier
) {
    Canvas(modifier = modifier.size(size)) {
        val u = this.size.minDimension / 64f

        // 1. Companion Star (Transparent background — zero outer dark box)
        val smallStar = Path().apply {
            moveTo(14.2f * u, 14.8f * u)
            quadraticBezierTo(14.2f * u, 17.2f * u, 16.6f * u, 17.2f * u)
            quadraticBezierTo(14.2f * u, 17.2f * u, 14.2f * u, 19.6f * u)
            quadraticBezierTo(14.2f * u, 17.2f * u, 11.8f * u, 17.2f * u)
            quadraticBezierTo(14.2f * u, 17.2f * u, 14.2f * u, 14.8f * u)
            close()
        }
        drawPath(smallStar, color = Color(0xBDFFE6C6))

        val orbitPivot = Offset(31.2f * u, 35.6f * u)
        val orbitTopLeft = Offset((31.2f - 25.8f) * u, (35.6f - 8.2f) * u)
        val orbitSize = Size(51.6f * u, 16.4f * u)
        val orbitBrush = Brush.linearGradient(
            0.0f to Color(0xFF6A564E),
            0.48f to Color(0xFFDEA976),
            1.0f to Color(0xFFFFF2DC),
            start = Offset(31.2f * u, 27.2f * u),
            end = Offset(31.2f * u, 44.0f * u)
        )

        // 3. Seamless 3D Orbital Ring (Back Pass)
        rotate(degrees = -22f, pivot = orbitPivot) {
            drawOval(
                brush = orbitBrush,
                topLeft = orbitTopLeft,
                size = orbitSize,
                style = Stroke(width = 2.3f * u)
            )
        }

        // 4. Sculpted Porcelain-Champagne Mochi Spirit
        val mochiPath = Path().apply {
            moveTo(31.2f * u, 14.4f * u)
            cubicTo(43.4f * u, 14.4f * u, 50.4f * u, 24.5f * u, 50.4f * u, 36.8f * u)
            cubicTo(50.4f * u, 47.1f * u, 42.8f * u, 52.4f * u, 31.2f * u, 52.4f * u)
            cubicTo(19.6f * u, 52.4f * u, 12.0f * u, 47.1f * u, 12.0f * u, 36.8f * u)
            cubicTo(12.0f * u, 24.5f * u, 19.0f * u, 14.4f * u, 31.2f * u, 14.4f * u)
            close()
        }
        clipPath(mochiPath) {
            drawPath(
                path = mochiPath,
                brush = Brush.radialGradient(
                    0.0f to Color.White,
                    0.54f to Color(0xFFF9F0E6),
                    1.0f to Color(0xFFDCC1A3),
                    center = Offset(24.5f * u, 21.5f * u),
                    radius = 33f * u
                )
            )
            drawCircle(
                brush = Brush.radialGradient(
                    0.0f to Color(0x85FFB36B),
                    0.65f to Color(0x26FFB36B),
                    1.0f to Color.Transparent,
                    center = Offset(39f * u, 43f * u),
                    radius = 20f * u
                ),
                radius = 20f * u,
                center = Offset(39f * u, 43f * u)
            )
            // Twin Obsidian Capsule Eyes + Specular Catchlights
            val eyeSize = Size(5.2f * u, 11.4f * u)
            val eyeCorner = CornerRadius(2.6f * u, 2.6f * u)
            drawRoundRect(
                color = Color(0xFF101524),
                topLeft = Offset(21.8f * u, 26.6f * u),
                size = eyeSize,
                cornerRadius = eyeCorner
            )
            drawCircle(
                color = Color(0xF2FFFFFF),
                radius = 1.25f * u,
                center = Offset(23.7f * u, 29.4f * u)
            )
            drawRoundRect(
                color = Color(0xFF101524),
                topLeft = Offset(35.4f * u, 26.6f * u),
                size = eyeSize,
                cornerRadius = eyeCorner
            )
            drawCircle(
                color = Color(0xF2FFFFFF),
                radius = 1.25f * u,
                center = Offset(37.3f * u, 29.4f * u)
            )
        }
        drawPath(
            path = mochiPath,
            color = Color(0x3864748B),
            style = Stroke(width = 0.9f * u)
        )

        // 5. Orbital Ring Front Crossing
        rotate(degrees = -22f, pivot = orbitPivot) {
            clipRect(
                left = 0f,
                top = 35.6f * u,
                right = 64f * u,
                bottom = 64f * u
            ) {
                drawOval(
                    brush = orbitBrush,
                    topLeft = orbitTopLeft,
                    size = orbitSize,
                    style = Stroke(width = 2.3f * u)
                )
            }
        }

        // 6. Vesper Evening Star (长庚星)
        val vesperStar = Path().apply {
            moveTo(50.2f * u, 6.8f * u)
            quadraticBezierTo(50.2f * u, 14.5f * u, 57.9f * u, 14.5f * u)
            quadraticBezierTo(50.2f * u, 14.5f * u, 50.2f * u, 22.2f * u)
            quadraticBezierTo(50.2f * u, 14.5f * u, 42.5f * u, 14.5f * u)
            quadraticBezierTo(50.2f * u, 14.5f * u, 50.2f * u, 6.8f * u)
            close()
        }
        drawPath(
            path = vesperStar,
            brush = Brush.linearGradient(
                0.0f to Color.White,
                0.52f to Color(0xFFFFE5BF),
                1.0f to Color(0xFFF2B880),
                start = Offset(44f * u, 8.5f * u),
                end = Offset(56f * u, 20.5f * u)
            )
        )
    }
}
