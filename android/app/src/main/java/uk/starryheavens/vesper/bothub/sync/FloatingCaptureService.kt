package uk.starryheavens.vesper.bothub.sync

import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.PixelFormat
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.provider.Settings
import android.util.DisplayMetrics
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import android.widget.FrameLayout
import android.widget.ImageView
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import kotlin.math.abs

class FloatingCaptureService : Service() {

    private var windowManager: WindowManager? = null
    private var rootView: FrameLayout? = null
    private var layoutParams: WindowManager.LayoutParams? = null
    private lateinit var prefs: BotHubPrefs
    private var dockedRight: Boolean = true
    private var temporarilyHidden: Boolean = false

    override fun onCreate() {
        super.onCreate()
        activeInstance = this
        prefs = BotHubPrefs(applicationContext)
        dockedRight = prefs.floatingBallEdgeRight
        windowManager = getSystemService(Context.WINDOW_SERVICE) as? WindowManager
        if (!canDrawOverlays(this) || !prefs.floatingBallEnabled) {
            stopSelf()
            return
        }
        attachOverlayView()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!canDrawOverlays(this) || !prefs.floatingBallEnabled) {
            stopSelf()
            return START_NOT_STICKY
        }
        when (intent?.action) {
            ACTION_SET_HIDDEN -> {
                applyHiddenState(intent.getBooleanExtra(EXTRA_HIDDEN, false))
            }
            else -> {
                if (rootView == null) {
                    attachOverlayView()
                } else {
                    rootView?.visibility = if (temporarilyHidden) View.GONE else View.VISIBLE
                }
            }
        }
        return START_STICKY
    }

    private fun applyHiddenState(hidden: Boolean) {
        temporarilyHidden = hidden
        mainHandler.post {
            rootView?.visibility = if (temporarilyHidden) View.GONE else View.VISIBLE
        }
    }

    override fun onDestroy() {
        if (activeInstance === this) {
            activeInstance = null
        }
        detachOverlayView()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun dp(value: Float): Int = (value * resources.displayMetrics.density + 0.5f).toInt()

    private fun screenDimensions(): Pair<Int, Int> {
        val wm = windowManager
        if (wm != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val bounds = wm.currentWindowMetrics.bounds
            return bounds.width() to bounds.height()
        }
        val dm: DisplayMetrics = resources.displayMetrics
        return dm.widthPixels to dm.heightPixels
    }

    private fun buildPillBackground(isRightEdge: Boolean): GradientDrawable {
        val r = dp(18f).toFloat()
        val radii = if (isRightEdge) {
            floatArrayOf(r, r, 0f, 0f, 0f, 0f, r, r)
        } else {
            floatArrayOf(0f, 0f, r, r, r, r, 0f, 0f)
        }
        return GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadii = radii
            setColor(Color.parseColor("#D9181A20"))
            setStroke(dp(1f), Color.parseColor("#66E2E8F0"))
        }
    }

    private fun attachOverlayView() {
        val wm = windowManager ?: return
        if (rootView != null) return

        val (screenW, screenH) = screenDimensions()
        val pillW = dp(36f)
        val pillH = dp(48f)
        val initialY = (screenH * prefs.floatingBallYFraction).toInt()
            .coerceIn(dp(72f), (screenH - pillH - dp(72f)).coerceAtLeast(dp(72f)))

        val container = FrameLayout(this).apply {
            background = buildPillBackground(dockedRight)
            alpha = 0.78f
            elevation = dp(6f).toFloat()
            contentDescription = "BotHub 快捷收藏"
        }

        val iconView = ImageView(this).apply {
            setImageResource(R.drawable.ic_notification)
            imageTintList = ColorStateList.valueOf(Color.parseColor("#F8FAFC"))
            scaleType = ImageView.ScaleType.FIT_CENTER
        }
        val iconSize = dp(20f)
        container.addView(
            iconView,
            FrameLayout.LayoutParams(iconSize, iconSize, Gravity.CENTER)
        )

        val params = WindowManager.LayoutParams(
            pillW,
            pillH,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = if (dockedRight) screenW - pillW else 0
            y = initialY
        }

        val touchSlop = dp(10f)
        var downRawX = 0f
        var downRawY = 0f
        var startX = 0
        var startY = 0
        var downTime = 0L
        var dragging = false

        container.setOnTouchListener { view, event ->
            val lp = layoutParams ?: return@setOnTouchListener false
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    downRawX = event.rawX
                    downRawY = event.rawY
                    startX = lp.x
                    startY = lp.y
                    downTime = System.currentTimeMillis()
                    dragging = false
                    view.alpha = 1.0f
                    true
                }
                MotionEvent.ACTION_MOVE -> {
                    val dx = (event.rawX - downRawX).toInt()
                    val dy = (event.rawY - downRawY).toInt()
                    if (!dragging && (abs(dx) > touchSlop || abs(dy) > touchSlop)) {
                        dragging = true
                    }
                    if (dragging) {
                        val (curW, curH) = screenDimensions()
                        lp.x = (startX + dx).coerceIn(0, (curW - pillW).coerceAtLeast(0))
                        lp.y = (startY + dy).coerceIn(dp(48f), (curH - pillH - dp(48f)).coerceAtLeast(dp(48f)))
                        runCatching { wm.updateViewLayout(view, lp) }
                    }
                    true
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                    view.alpha = 0.78f
                    val elapsed = System.currentTimeMillis() - downTime
                    if (!dragging && event.actionMasked == MotionEvent.ACTION_UP && elapsed < 450L) {
                        view.performClick()
                        runCatching {
                            startActivity(AppIntents.createClipboardCaptureIntent(applicationContext))
                        }
                    } else {
                        val (curW, curH) = screenDimensions()
                        val centerX = lp.x + pillW / 2
                        dockedRight = centerX >= curW / 2
                        lp.x = if (dockedRight) (curW - pillW).coerceAtLeast(0) else 0
                        view.background = buildPillBackground(dockedRight)
                        runCatching { wm.updateViewLayout(view, lp) }
                        prefs.floatingBallEdgeRight = dockedRight
                        if (curH > 0) {
                            prefs.floatingBallYFraction = lp.y.toFloat() / curH.toFloat()
                        }
                    }
                    true
                }
                else -> false
            }
        }

        container.visibility = if (temporarilyHidden) View.GONE else View.VISIBLE
        val added = runCatching {
            wm.addView(container, params)
            true
        }.getOrDefault(false)
        if (added) {
            rootView = container
            layoutParams = params
        } else {
            stopSelf()
        }
    }

    private fun detachOverlayView() {
        val view = rootView ?: return
        rootView = null
        layoutParams = null
        runCatching { windowManager?.removeViewImmediate(view) }
    }

    companion object {
        private const val ACTION_SET_HIDDEN = "uk.starryheavens.vesper.bothub.FLOATING_SET_HIDDEN"
        private const val EXTRA_HIDDEN = "extra_hidden"

        private val mainHandler = Handler(Looper.getMainLooper())

        @Volatile
        private var activeInstance: FloatingCaptureService? = null

        fun canDrawOverlays(context: Context): Boolean {
            return runCatching { Settings.canDrawOverlays(context.applicationContext) }.getOrDefault(false)
        }

        fun openOverlayPermissionSettings(context: Context) {
            val intent = Intent(
                Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                Uri.parse("package:${context.packageName}")
            ).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            runCatching { context.startActivity(intent) }
        }

        fun syncWithPrefs(context: Context, prefs: BotHubPrefs, appInForeground: Boolean = false) {
            val appContext = context.applicationContext
            if (prefs.floatingBallEnabled && canDrawOverlays(appContext)) {
                val running = activeInstance
                if (running != null) {
                    running.applyHiddenState(appInForeground)
                    return
                }
                val intent = Intent(appContext, FloatingCaptureService::class.java).apply {
                    action = ACTION_SET_HIDDEN
                    putExtra(EXTRA_HIDDEN, appInForeground)
                }
                runCatching { appContext.startService(intent) }
            } else {
                runCatching { appContext.stopService(Intent(appContext, FloatingCaptureService::class.java)) }
            }
        }

        fun setTemporarilyHidden(context: Context, hidden: Boolean) {
            val appContext = context.applicationContext
            val prefs = BotHubPrefs(appContext)
            if (!prefs.floatingBallEnabled || !canDrawOverlays(appContext)) return
            val running = activeInstance
            if (running != null) {
                running.applyHiddenState(hidden)
                return
            }
            val intent = Intent(appContext, FloatingCaptureService::class.java).apply {
                action = ACTION_SET_HIDDEN
                putExtra(EXTRA_HIDDEN, hidden)
            }
            runCatching { appContext.startService(intent) }
        }
    }
}
