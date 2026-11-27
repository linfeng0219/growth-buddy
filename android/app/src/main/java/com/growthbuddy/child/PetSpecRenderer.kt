package com.growthbuddy.child

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Matrix
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RadialGradient
import android.graphics.RectF
import android.graphics.Shader
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import kotlin.math.PI
import kotlin.math.acos
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlin.math.sqrt

/**
 * 解析并缓存与 Web 端共享的 `pet-spec.json` 矢量规范，确保双端宠物几何轮廓、渐变与动效完全同源。
 */
object PetSpecStore {

    data class SpecBox(val left: Float, val top: Float, val right: Float, val bottom: Float) {
        fun width(): Float = right - left
        fun height(): Float = bottom - top
    }

    data class ViewBoxSpec(val w: Float, val h: Float, val originX: Float, val originY: Float)
    data class BreatheSpec(val durationMs: Long, val originX: Float, val originY: Float, val maxScaleX: Float, val minScaleY: Float)
    data class BlinkSpec(val durationMs: Long, val holdRatio: Float, val closeRatio: Float, val minScaleY: Float)
    data class EggWobbleSpec(val durationMs: Long, val pivotX: Float, val pivotY: Float, val keyframes: List<Pair<Float, Float>>)
    data class AnimBundle(val breathe: BreatheSpec, val blink: BlinkSpec, val eggWobble: EggWobbleSpec)

    data class GradStop(val offset: Float, val colorToken: String, val alpha: Float)
    data class GradSpec(val cx: Float, val cy: Float, val r: Float, val stops: List<GradStop>)

    data class PedestalPalette(val fill: String, val stroke: String, val dot: String)
    data class PedestalSpec(val palettes: Map<String, PedestalPalette>, val shapes: List<ShapeSpec>)

    sealed class PathCmd {
        data class Move(val x: Float, val y: Float) : PathCmd()
        data class Line(val x: Float, val y: Float) : PathCmd()
        data class Quad(val x1: Float, val y1: Float, val x: Float, val y: Float) : PathCmd()
        data class Arc(
            val x0: Float, val y0: Float,
            val rx: Float, val ry: Float, val xAxisRot: Float,
            val largeArc: Boolean, val sweep: Boolean,
            val x: Float, val y: Float
        ) : PathCmd()
        data object Close : PathCmd()
    }

    sealed class ShapeSpec {
        abstract val whenCond: String?

        data class Group(
            val anim: String?,
            val children: List<ShapeSpec>,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class Ellipse(
            val cx: Float, val cy: Float, val rx: Float, val ry: Float,
            val fill: String?, val silFill: String?, val stroke: String?, val sw: Float,
            val opacity: Float, val rotDeg: Float?, val rotX: Float, val rotY: Float,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class Circle(
            val cx: Float, val cy: Float, val r: Float,
            val fill: String?, val silFill: String?, val stroke: String?, val sw: Float,
            val opacity: Float,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class RoundRect(
            val x: Float, val y: Float, val w: Float, val h: Float, val rx: Float,
            val fill: String?, val silFill: String?, val stroke: String?, val sw: Float,
            val opacity: Float,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class VectorPath(
            val rawD: String,
            val cmds: List<PathCmd>,
            val bounds: SpecBox,
            val fill: String?, val silFill: String?, val stroke: String?, val sw: Float,
            val cap: String?, val opacity: Float, val rotDeg: Float?, val rotX: Float, val rotY: Float,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class Eyes(
            val x1: Float, val x2: Float, val y: Float,
            override val whenCond: String? = null
        ) : ShapeSpec()

        data class TextLabel(
            val x: Float, val y: Float, val text: String, val size: Float, val fill: String?,
            override val whenCond: String? = null
        ) : ShapeSpec()
    }

    data class HaloSpec(val cx: Float, val cy: Float, val r: Float, val fill: String, val opacity: Float)
    data class StageRule(
        val scale: Float, val anchorX: Float, val anchorY: Float,
        val overrideAcc: String?, val wingToTail: String?, val crownIfNoAcc: Boolean
    )

    data class MainStageSpec(
        val base: String, val halo: HaloSpec?, val groupAnim: String?, val shapes: List<ShapeSpec>
    )

    data class Stage1Spec(
        val clip: SpecBox,
        val ears: Map<String, List<ShapeSpec>>,
        val ahoge: List<ShapeSpec>,
        val head: List<ShapeSpec>,
        val mouths: Map<String, List<ShapeSpec>>,
        val shell: List<ShapeSpec>
    )

    data class SpeciesTemplateSpec(
        val haloByLv: Map<Int, HaloSpec>,
        val stageRules: Map<Int, StageRule>,
        val stage0Shapes: List<ShapeSpec>,
        val stage1: Stage1Spec,
        val tails: Map<String, List<ShapeSpec>>,
        val ears: Map<String, List<ShapeSpec>>,
        val ahoge: List<ShapeSpec>,
        val body: List<ShapeSpec>,
        val face: List<ShapeSpec>,
        val mouths: Map<String, List<ShapeSpec>>,
        val silhouetteMark: List<ShapeSpec>,
        val accessories: Map<String, List<ShapeSpec>>
    )

    data class PetSpecDoc(
        val viewBox: ViewBoxSpec,
        val animations: AnimBundle,
        val gradients: Map<String, GradSpec>,
        val pedestals: PedestalSpec,
        val mainPetStages: Map<Int, MainStageSpec>,
        val speciesTemplate: SpeciesTemplateSpec
    )

    @Volatile
    private var cached: PetSpecDoc? = null
    private val lock = Any()

    fun get(context: Context? = null): PetSpecDoc {
        cached?.let { return it }
        synchronized(lock) {
            cached?.let { return it }
            val jsonText = loadRawJson(context)
            val parsed = parseDoc(JSONObject(jsonText))
            cached = parsed
            return parsed
        }
    }

    internal fun parseFromJsonString(jsonText: String): PetSpecDoc = parseDoc(JSONObject(jsonText))

    private fun loadRawJson(context: Context?): String {
        if (context != null) {
            runCatching {
                context.applicationContext.assets.open("pet-spec.json").bufferedReader(Charsets.UTF_8).use {
                    return it.readText()
                }
            }
        }
        PetSpecStore::class.java.classLoader?.getResourceAsStream("pet-spec.json")?.use { stream ->
            return stream.bufferedReader(Charsets.UTF_8).use { it.readText() }
        }
        for (candidate in listOf(
            File("server/public/shared/pet-spec.json"),
            File("../server/public/shared/pet-spec.json"),
            File("../../server/public/shared/pet-spec.json")
        )) {
            if (candidate.isFile) {
                return candidate.readText(Charsets.UTF_8)
            }
        }
        error("Unable to locate pet-spec.json in assets, resources, or workspace")
    }

    private fun parseDoc(root: JSONObject): PetSpecDoc {
        val vbObj = root.getJSONObject("viewBox")
        val viewBox = ViewBoxSpec(
            w = vbObj.getDouble("w").toFloat(),
            h = vbObj.getDouble("h").toFloat(),
            originX = vbObj.getDouble("originX").toFloat(),
            originY = vbObj.getDouble("originY").toFloat()
        )

        val animObj = root.getJSONObject("animations")
        val bObj = animObj.getJSONObject("breathe")
        val blObj = animObj.getJSONObject("blink")
        val ewObj = animObj.getJSONObject("eggWobble")
        val ewKeys = ewObj.getJSONArray("keyframes")
        val kfList = (0 until ewKeys.length()).map { idx ->
            val pair = ewKeys.getJSONArray(idx)
            pair.getDouble(0).toFloat() to pair.getDouble(1).toFloat()
        }
        val animBundle = AnimBundle(
            breathe = BreatheSpec(
                durationMs = bObj.getLong("durationMs"),
                originX = bObj.getDouble("originX").toFloat(),
                originY = bObj.getDouble("originY").toFloat(),
                maxScaleX = bObj.getDouble("maxScaleX").toFloat(),
                minScaleY = bObj.getDouble("minScaleY").toFloat()
            ),
            blink = BlinkSpec(
                durationMs = blObj.getLong("durationMs"),
                holdRatio = blObj.getDouble("holdRatio").toFloat(),
                closeRatio = blObj.getDouble("closeRatio").toFloat(),
                minScaleY = blObj.getDouble("minScaleY").toFloat()
            ),
            eggWobble = EggWobbleSpec(
                durationMs = ewObj.getLong("durationMs"),
                pivotX = ewObj.getDouble("pivotX").toFloat(),
                pivotY = ewObj.getDouble("pivotY").toFloat(),
                keyframes = kfList
            )
        )

        val gradsObj = root.getJSONObject("gradients")
        val grads = mutableMapOf<String, GradSpec>()
        for (key in gradsObj.keys()) {
            val gObj = gradsObj.getJSONObject(key)
            val sArr = gObj.getJSONArray("stops")
            val stops = (0 until sArr.length()).map { i ->
                val item = sArr.getJSONArray(i)
                GradStop(
                    offset = item.getDouble(0).toFloat(),
                    colorToken = item.getString(1),
                    alpha = if (item.length() > 2) item.getDouble(2).toFloat() else 1f
                )
            }
            grads[key] = GradSpec(
                cx = gObj.getDouble("cx").toFloat(),
                cy = gObj.getDouble("cy").toFloat(),
                r = gObj.getDouble("r").toFloat(),
                stops = stops
            )
        }

        val pedObj = root.getJSONObject("pedestals")
        val palObj = pedObj.getJSONObject("palettes")
        val palettes = mutableMapOf<String, PedestalPalette>()
        for (k in palObj.keys()) {
            val p = palObj.getJSONObject(k)
            palettes[k] = PedestalPalette(p.getString("fill"), p.getString("stroke"), p.getString("dot"))
        }
        val pedestals = PedestalSpec(palettes = palettes, shapes = parseShapes(pedObj.getJSONArray("shapes")))

        val mainObj = root.getJSONObject("mainPetStages")
        val mainStages = mutableMapOf<Int, MainStageSpec>()
        for (k in mainObj.keys()) {
            val stObj = mainObj.getJSONObject(k)
            val halo = stObj.optJSONObject("halo")?.let {
                HaloSpec(
                    cx = it.getDouble("cx").toFloat(),
                    cy = it.getDouble("cy").toFloat(),
                    r = it.getDouble("r").toFloat(),
                    fill = it.getString("fill"),
                    opacity = it.getDouble("opacity").toFloat()
                )
            }
            mainStages[k.toInt()] = MainStageSpec(
                base = stObj.optString("base", "grass"),
                halo = halo,
                groupAnim = stObj.optString("groupAnim", "").ifEmpty { null },
                shapes = parseShapes(stObj.getJSONArray("shapes"))
            )
        }

        val tplObj = root.getJSONObject("speciesTemplate")
        val haloObj = tplObj.getJSONObject("haloByLv")
        val haloMap = mutableMapOf<Int, HaloSpec>()
        for (k in haloObj.keys()) {
            val h = haloObj.getJSONObject(k)
            haloMap[k.toInt()] = HaloSpec(
                cx = h.getDouble("cx").toFloat(),
                cy = h.getDouble("cy").toFloat(),
                r = h.getDouble("r").toFloat(),
                fill = h.getString("fill"),
                opacity = h.getDouble("opacity").toFloat()
            )
        }
        val rulesObj = tplObj.getJSONObject("stageRules")
        val rulesMap = mutableMapOf<Int, StageRule>()
        for (k in rulesObj.keys()) {
            val r = rulesObj.getJSONObject(k)
            rulesMap[k.toInt()] = StageRule(
                scale = r.getDouble("scale").toFloat(),
                anchorX = r.getDouble("anchorX").toFloat(),
                anchorY = r.getDouble("anchorY").toFloat(),
                overrideAcc = r.optString("overrideAcc", "").ifEmpty { null },
                wingToTail = r.optString("wingToTail", "").ifEmpty { null },
                crownIfNoAcc = r.optBoolean("crownIfNoAcc", false)
            )
        }
        val st1Obj = tplObj.getJSONObject("stage1")
        val clipArr = st1Obj.getJSONArray("clip")
        val clipRect = SpecBox(
            clipArr.getDouble(0).toFloat(),
            clipArr.getDouble(1).toFloat(),
            (clipArr.getDouble(0) + clipArr.getDouble(2)).toFloat(),
            (clipArr.getDouble(1) + clipArr.getDouble(3)).toFloat()
        )
        val stage1 = Stage1Spec(
            clip = clipRect,
            ears = parseShapeMap(st1Obj.getJSONObject("ears")),
            ahoge = parseShapes(st1Obj.getJSONArray("ahoge")),
            head = parseShapes(st1Obj.getJSONArray("head")),
            mouths = parseShapeMap(st1Obj.getJSONObject("mouths")),
            shell = parseShapes(st1Obj.getJSONArray("shell"))
        )

        val speciesTemplate = SpeciesTemplateSpec(
            haloByLv = haloMap,
            stageRules = rulesMap,
            stage0Shapes = parseShapes(tplObj.getJSONArray("stage0Shapes")),
            stage1 = stage1,
            tails = parseShapeMap(tplObj.getJSONObject("tails")),
            ears = parseShapeMap(tplObj.getJSONObject("ears")),
            ahoge = parseShapes(tplObj.getJSONArray("ahoge")),
            body = parseShapes(tplObj.getJSONArray("body")),
            face = parseShapes(tplObj.getJSONArray("face")),
            mouths = parseShapeMap(tplObj.getJSONObject("mouths")),
            silhouetteMark = parseShapes(tplObj.getJSONArray("silhouetteMark")),
            accessories = parseShapeMap(tplObj.getJSONObject("accessories"))
        )

        return PetSpecDoc(
            viewBox = viewBox,
            animations = animBundle,
            gradients = grads,
            pedestals = pedestals,
            mainPetStages = mainStages,
            speciesTemplate = speciesTemplate
        )
    }

    private fun parseShapeMap(obj: JSONObject): Map<String, List<ShapeSpec>> {
        val out = mutableMapOf<String, List<ShapeSpec>>()
        for (k in obj.keys()) {
            out[k] = parseShapes(obj.getJSONArray(k))
        }
        return out
    }

    private fun parseShapes(arr: JSONArray): List<ShapeSpec> {
        val list = ArrayList<ShapeSpec>(arr.length())
        for (i in 0 until arr.length()) {
            val o = arr.getJSONObject(i)
            val whenCond = o.optString("when", "").ifEmpty { null }
            val rotArr = o.optJSONArray("rot")
            val rotDeg = if (rotArr != null && rotArr.length() == 3) rotArr.getDouble(0).toFloat() else null
            val rotX = if (rotArr != null && rotArr.length() == 3) rotArr.getDouble(1).toFloat() else 0f
            val rotY = if (rotArr != null && rotArr.length() == 3) rotArr.getDouble(2).toFloat() else 0f
            val fill = o.optString("fill", "").ifEmpty { null }
            val silFill = o.optString("silFill", "").ifEmpty { null }
            val stroke = o.optString("stroke", "").ifEmpty { null }
            val sw = o.optDouble("sw", 2.0).toFloat()
            val op = o.optDouble("opacity", 1.0).toFloat()

            when (o.getString("type")) {
                "group" -> list.add(
                    ShapeSpec.Group(
                        anim = o.optString("anim", "").ifEmpty { null },
                        children = parseShapes(o.getJSONArray("children")),
                        whenCond = whenCond
                    )
                )
                "ellipse" -> list.add(
                    ShapeSpec.Ellipse(
                        cx = o.getDouble("cx").toFloat(),
                        cy = o.getDouble("cy").toFloat(),
                        rx = o.getDouble("rx").toFloat(),
                        ry = o.getDouble("ry").toFloat(),
                        fill = fill, silFill = silFill, stroke = stroke, sw = sw,
                        opacity = op, rotDeg = rotDeg, rotX = rotX, rotY = rotY, whenCond = whenCond
                    )
                )
                "circle" -> list.add(
                    ShapeSpec.Circle(
                        cx = o.getDouble("cx").toFloat(),
                        cy = o.getDouble("cy").toFloat(),
                        r = o.getDouble("r").toFloat(),
                        fill = fill, silFill = silFill, stroke = stroke, sw = sw,
                        opacity = op, whenCond = whenCond
                    )
                )
                "rect" -> list.add(
                    ShapeSpec.RoundRect(
                        x = o.getDouble("x").toFloat(),
                        y = o.getDouble("y").toFloat(),
                        w = o.getDouble("w").toFloat(),
                        h = o.getDouble("h").toFloat(),
                        rx = o.optDouble("rx", 0.0).toFloat(),
                        fill = fill, silFill = silFill, stroke = stroke, sw = sw,
                        opacity = op, whenCond = whenCond
                    )
                )
                "path" -> {
                    val d = o.getString("d")
                    val cmds = SvgPathParser.parse(d)
                    val bounds = SvgPathParser.computeBounds(cmds)
                    list.add(
                        ShapeSpec.VectorPath(
                            rawD = d,
                            cmds = cmds,
                            bounds = bounds,
                            fill = fill, silFill = silFill, stroke = stroke, sw = sw,
                            cap = o.optString("cap", "").ifEmpty { null },
                            opacity = op, rotDeg = rotDeg, rotX = rotX, rotY = rotY, whenCond = whenCond
                        )
                    )
                }
                "eyes" -> list.add(
                    ShapeSpec.Eyes(
                        x1 = o.getDouble("x1").toFloat(),
                        x2 = o.getDouble("x2").toFloat(),
                        y = o.getDouble("y").toFloat(),
                        whenCond = whenCond
                    )
                )
                "text" -> list.add(
                    ShapeSpec.TextLabel(
                        x = o.getDouble("x").toFloat(),
                        y = o.getDouble("y").toFloat(),
                        text = o.getString("text"),
                        size = o.optDouble("size", 38.0).toFloat(),
                        fill = fill,
                        whenCond = whenCond
                    )
                )
            }
        }
        return list
    }
}

internal object SvgPathParser {
    fun parse(d: String): List<PetSpecStore.PathCmd> {
        val out = ArrayList<PetSpecStore.PathCmd>()
        var i = 0
        val n = d.length
        var curX = 0f
        var curY = 0f
        var startX = 0f
        var startY = 0f
        var cmd = ' '

        fun skipSep() {
            while (i < n && (d[i] == ' ' || d[i] == ',' || d[i] == '\t' || d[i] == '\n' || d[i] == '\r')) i++
        }

        fun hasNumber(): Boolean {
            skipSep()
            if (i >= n) return false
            val c = d[i]
            return c in '0'..'9' || c == '-' || c == '+' || c == '.'
        }

        fun nextFloat(): Float {
            skipSep()
            val start = i
            if (i < n && (d[i] == '-' || d[i] == '+')) i++
            var seenDot = false
            while (i < n) {
                val c = d[i]
                if (c in '0'..'9') {
                    i++
                } else if (c == '.' && !seenDot) {
                    seenDot = true
                    i++
                } else {
                    break
                }
            }
            return d.substring(start, i).toFloat()
        }

        fun nextFlag(): Boolean {
            skipSep()
            val ch = d[i++]
            return ch == '1'
        }

        while (true) {
            skipSep()
            if (i >= n) break
            val c = d[i]
            if ((c in 'A'..'Z') || (c in 'a'..'z')) {
                cmd = c
                i++
                if (cmd == 'Z' || cmd == 'z') {
                    out.add(PetSpecStore.PathCmd.Close)
                    curX = startX
                    curY = startY
                    continue
                }
            } else if (cmd == ' ') {
                break
            }

            when (cmd) {
                'M', 'm' -> {
                    val first = hasNumber()
                    if (first) {
                        var nx = nextFloat()
                        var ny = nextFloat()
                        if (cmd == 'm') {
                            nx += curX
                            ny += curY
                        }
                        out.add(PetSpecStore.PathCmd.Move(nx, ny))
                        curX = nx
                        curY = ny
                        startX = nx
                        startY = ny
                        cmd = if (cmd == 'm') 'l' else 'L'
                    }
                }
                'L', 'l' -> {
                    while (hasNumber()) {
                        var nx = nextFloat()
                        var ny = nextFloat()
                        if (cmd == 'l') {
                            nx += curX
                            ny += curY
                        }
                        out.add(PetSpecStore.PathCmd.Line(nx, ny))
                        curX = nx
                        curY = ny
                    }
                }
                'H', 'h' -> {
                    while (hasNumber()) {
                        var nx = nextFloat()
                        if (cmd == 'h') nx += curX
                        out.add(PetSpecStore.PathCmd.Line(nx, curY))
                        curX = nx
                    }
                }
                'V', 'v' -> {
                    while (hasNumber()) {
                        var ny = nextFloat()
                        if (cmd == 'v') ny += curY
                        out.add(PetSpecStore.PathCmd.Line(curX, ny))
                        curY = ny
                    }
                }
                'Q', 'q' -> {
                    while (hasNumber()) {
                        var x1 = nextFloat()
                        var y1 = nextFloat()
                        var x = nextFloat()
                        var y = nextFloat()
                        if (cmd == 'q') {
                            x1 += curX; y1 += curY; x += curX; y += curY
                        }
                        out.add(PetSpecStore.PathCmd.Quad(x1, y1, x, y))
                        curX = x
                        curY = y
                    }
                }
                'A', 'a' -> {
                    while (hasNumber()) {
                        val rx = nextFloat()
                        val ry = nextFloat()
                        val rot = nextFloat()
                        val large = nextFlag()
                        val sweep = nextFlag()
                        var x = nextFloat()
                        var y = nextFloat()
                        if (cmd == 'a') {
                            x += curX; y += curY
                        }
                        out.add(PetSpecStore.PathCmd.Arc(curX, curY, rx, ry, rot, large, sweep, x, y))
                        curX = x
                        curY = y
                    }
                }
                else -> i++
            }
        }
        return out
    }

    fun computeBounds(cmds: List<PetSpecStore.PathCmd>): PetSpecStore.SpecBox {
        var minX = Float.POSITIVE_INFINITY
        var minY = Float.POSITIVE_INFINITY
        var maxX = Float.NEGATIVE_INFINITY
        var maxY = Float.NEGATIVE_INFINITY
        fun expand(x: Float, y: Float) {
            if (x < minX) minX = x
            if (y < minY) minY = y
            if (x > maxX) maxX = x
            if (y > maxY) maxY = y
        }
        for (c in cmds) {
            when (c) {
                is PetSpecStore.PathCmd.Move -> expand(c.x, c.y)
                is PetSpecStore.PathCmd.Line -> expand(c.x, c.y)
                is PetSpecStore.PathCmd.Quad -> {
                    expand(c.x1, c.y1)
                    expand(c.x, c.y)
                }
                is PetSpecStore.PathCmd.Arc -> {
                    expand(c.x0, c.y0)
                    expand(c.x, c.y)
                    val cx = (c.x0 + c.x) * 0.5f
                    val cy = (c.y0 + c.y) * 0.5f
                    expand(cx - c.rx, cy - c.ry)
                    expand(cx + c.rx, cy + c.ry)
                }
                PetSpecStore.PathCmd.Close -> {}
            }
        }
        if (minX.isInfinite()) return PetSpecStore.SpecBox(0f, 0f, 1f, 1f)
        return PetSpecStore.SpecBox(minX, minY, maxX, maxY)
    }

    fun buildAndroidPath(
        cmds: List<PetSpecStore.PathCmd>,
        ox: Float, oy: Float, cx: Float, cy: Float, scale: Float
    ): Path {
        val p = Path()
        fun tx(x: Float) = cx + (x - ox) * scale
        fun ty(y: Float) = cy + (y - oy) * scale
        for (c in cmds) {
            when (c) {
                is PetSpecStore.PathCmd.Move -> p.moveTo(tx(c.x), ty(c.y))
                is PetSpecStore.PathCmd.Line -> p.lineTo(tx(c.x), ty(c.y))
                is PetSpecStore.PathCmd.Quad -> p.quadTo(tx(c.x1), ty(c.y1), tx(c.x), ty(c.y))
                is PetSpecStore.PathCmd.Arc -> addSvgArc(p, c, ox, oy, cx, cy, scale)
                PetSpecStore.PathCmd.Close -> p.close()
            }
        }
        return p
    }

    private fun addSvgArc(
        p: Path,
        arc: PetSpecStore.PathCmd.Arc,
        ox: Float, oy: Float, cx: Float, cy: Float, scale: Float
    ) {
        val x0 = arc.x0.toDouble()
        val y0 = arc.y0.toDouble()
        val x = arc.x.toDouble()
        val y = arc.y.toDouble()
        if (x0 == x && y0 == y) return
        var rx = kotlin.math.abs(arc.rx.toDouble())
        var ry = kotlin.math.abs(arc.ry.toDouble())
        if (rx == 0.0 || ry == 0.0) {
            p.lineTo(cx + (arc.x - ox) * scale, cy + (arc.y - oy) * scale)
            return
        }
        val phi = Math.toRadians(arc.xAxisRot.toDouble())
        val cosPhi = cos(phi)
        val sinPhi = sin(phi)
        val dx2 = (x0 - x) * 0.5
        val dy2 = (y0 - y) * 0.5
        val x1p = cosPhi * dx2 + sinPhi * dy2
        val y1p = -sinPhi * dx2 + cosPhi * dy2

        val lambda = (x1p * x1p) / (rx * rx) + (y1p * y1p) / (ry * ry)
        if (lambda > 1.0) {
            val s = sqrt(lambda)
            rx *= s
            ry *= s
        }
        val rxSq = rx * rx
        val rySq = ry * ry
        val x1pSq = x1p * x1p
        val y1pSq = y1p * y1p
        val denom = rxSq * y1pSq + rySq * x1pSq
        var factor = if (denom == 0.0) 0.0 else sqrt(max(0.0, (rxSq * rySq - denom) / denom))
        if (arc.largeArc == arc.sweep) factor = -factor
        val cxp = factor * (rx * y1p / ry)
        val cyp = factor * (-ry * x1p / rx)
        val acx = cosPhi * cxp - sinPhi * cyp + (x0 + x) * 0.5
        val acy = sinPhi * cxp + cosPhi * cyp + (y0 + y) * 0.5

        fun vecAngle(ux: Double, uy: Double, vx: Double, vy: Double): Double {
            val dot = ux * vx + uy * vy
            val len = sqrt((ux * ux + uy * uy) * (vx * vx + vy * vy))
            val ratio = (if (len == 0.0) 1.0 else dot / len).coerceIn(-1.0, 1.0)
            val ang = acos(ratio)
            return if (ux * vy - uy * vx < 0) -ang else ang
        }

        val ux = (x1p - cxp) / rx
        val uy = (y1p - cyp) / ry
        val vx = (-x1p - cxp) / rx
        val vy = (-y1p - cyp) / ry
        val theta1 = vecAngle(1.0, 0.0, ux, uy)
        var dTheta = vecAngle(ux, uy, vx, vy)
        if (!arc.sweep && dTheta > 0) dTheta -= 2.0 * PI
        if (arc.sweep && dTheta < 0) dTheta += 2.0 * PI

        val startDeg = Math.toDegrees(theta1).toFloat()
        val sweepDeg = Math.toDegrees(dTheta).toFloat()
        val scrCx = cx + (acx.toFloat() - ox) * scale
        val scrCy = cy + (acy.toFloat() - oy) * scale
        val scrRx = rx.toFloat() * scale
        val scrRy = ry.toFloat() * scale
        val oval = RectF(scrCx - scrRx, scrCy - scrRy, scrCx + scrRx, scrCy + scrRy)
        if (arc.xAxisRot == 0f) {
            p.arcTo(oval, startDeg, sweepDeg, false)
        } else {
            val seg = Path().apply { addArc(oval, startDeg, sweepDeg) }
            val mat = Matrix().apply { postRotate(arc.xAxisRot, scrCx, scrCy) }
            seg.transform(mat)
            p.addPath(seg)
        }
    }
}

/**
 * 依据 `PetSpecStore` 矢量指令在 Android Canvas 上 60fps 渲染宠物，保证与 Web SVG 引擎几何/动效同源。
 */
internal object PetSpecRenderer {

    private data class RenderCtx(
        val doc: PetSpecStore.PetSpecDoc,
        val cx: Float,
        val cy: Float,
        val scale: Float,
        val sleep: Boolean,
        val sil: Boolean,
        val breath: Float,
        val blinkScaleY: Float,
        val eggWobbleDeg: Float,
        val tokens: Map<String, Int>,
        val paint: Paint
    ) {
        val ox: Float get() = doc.viewBox.originX
        val oy: Float get() = doc.viewBox.originY
        fun tx(x: Float): Float = cx + (x - ox) * scale
        fun ty(y: Float): Float = cy + (y - oy) * scale
    }

    fun drawPedestal(canvas: Canvas, base: String, cx: Float, cy: Float, scale: Float, paint: Paint) {
        val doc = PetSpecStore.get()
        val ped = doc.pedestals
        val pal = ped.palettes[base] ?: ped.palettes["grass"] ?: return
        // DrawPedestal receives caller y for pedestal anchor (matching y=176 in viewBox)
        val drawCy = cy - (176f - doc.viewBox.originY) * scale
        val ctx = RenderCtx(
            doc = doc, cx = cx, cy = drawCy, scale = scale,
            sleep = false, sil = false, breath = 0f, blinkScaleY = 1f, eggWobbleDeg = 0f,
            tokens = mapOf(
                "@pedFill" to PetArtCanvas.parseColorSafe(pal.fill),
                "@pedStroke" to PetArtCanvas.parseColorSafe(pal.stroke),
                "@pedDot" to PetArtCanvas.parseColorSafe(pal.dot)
            ),
            paint = paint
        )
        drawShapes(canvas, ped.shapes, ctx)
    }

    fun drawMainPet(
        canvas: Canvas,
        lv: Int,
        cx: Float,
        cy: Float,
        scale: Float,
        sleep: Boolean,
        breath: Float,
        paint: Paint,
        blinkScaleY: Float = 1f,
        eggWobbleDeg: Float = 0f
    ) {
        val doc = PetSpecStore.get()
        val st = doc.mainPetStages[lv.coerceIn(0, 5)] ?: doc.mainPetStages[5] ?: return
        val ctx = RenderCtx(
            doc = doc, cx = cx, cy = cy, scale = scale,
            sleep = sleep, sil = false, breath = breath, blinkScaleY = blinkScaleY, eggWobbleDeg = eggWobbleDeg,
            tokens = emptyMap(), paint = paint
        )
        drawPedestalSpec(canvas, st.base, ctx)
        st.halo?.let { drawHalo(canvas, it, ctx) }
        drawShape(canvas, PetSpecStore.ShapeSpec.Group(st.groupAnim, st.shapes), ctx)
    }

    fun drawSpecies(
        canvas: Canvas,
        sp: SpeciesItem,
        cx: Float,
        cy: Float,
        scale: Float,
        silhouette: Boolean,
        drawBase: Boolean,
        sleep: Boolean,
        breath: Float,
        blinkScaleY: Float,
        halo: PetSpecStore.HaloSpec?,
        stageScale: Float?,
        stageAnchorX: Float,
        stageAnchorY: Float,
        paint: Paint
    ) {
        val doc = PetSpecStore.get()
        val tpl = doc.speciesTemplate
        val silColor = 0xFFBFB8AC.toInt()
        val bodyC = if (silhouette) silColor else PetArtCanvas.parseColorSafe(sp.color)
        val accC = when {
            silhouette -> silColor
            sp.acc == "scarf" -> 0xFFC95742.toInt()
            sp.acc == "bow" -> 0xFFE2758E.toInt()
            else -> 0xFFF2C44D.toInt()
        }
        val ctx = RenderCtx(
            doc = doc, cx = cx, cy = cy, scale = scale,
            sleep = sleep, sil = silhouette, breath = breath, blinkScaleY = blinkScaleY, eggWobbleDeg = 0f,
            tokens = mapOf("@body" to bodyC, "@acc" to accC),
            paint = paint
        )
        if (drawBase && !silhouette) {
            drawPedestalSpec(canvas, sp.base.ifBlank { "grass" }, ctx)
        }
        if (halo != null && !silhouette) {
            drawHalo(canvas, halo, ctx)
        }

        val tailList = tpl.tails[sp.tail] ?: emptyList()
        val earList = tpl.ears[sp.ear] ?: tpl.ears["round"] ?: emptyList()
        val inner = ArrayList<PetSpecStore.ShapeSpec>()
        inner.addAll(earList)
        if (sp.ahoge > 0) inner.addAll(tpl.ahoge)
        inner.addAll(tpl.body)
        if (!silhouette) {
            inner.addAll(tpl.face)
            inner.addAll(tpl.mouths[sp.mouth] ?: tpl.mouths["smile"] ?: emptyList())
        } else {
            inner.addAll(tpl.silhouetteMark)
        }
        inner.addAll(tpl.accessories[sp.acc] ?: emptyList())

        canvas.save()
        if (stageScale != null && stageScale != 1f) {
            canvas.scale(stageScale, stageScale, ctx.tx(stageAnchorX), ctx.ty(stageAnchorY))
        }
        drawShapes(canvas, tailList, ctx)
        drawShape(canvas, PetSpecStore.ShapeSpec.Group(if (silhouette) null else "breathe", inner), ctx)
        canvas.restore()
    }

    fun drawPetForStage(
        canvas: Canvas,
        lv: Int,
        cx: Float,
        cy: Float,
        scale: Float,
        sleep: Boolean,
        breath: Float,
        species: SpeciesItem?,
        paint: Paint,
        blinkScaleY: Float = 1f,
        eggWobbleDeg: Float = 0f
    ) {
        if (species == null || species.id == "b0") {
            drawMainPet(canvas, lv, cx, cy, scale, sleep, breath, paint, blinkScaleY, eggWobbleDeg)
            return
        }
        val doc = PetSpecStore.get()
        val tpl = doc.speciesTemplate
        val bodyTint = PetArtCanvas.parseColorSafe(species.color)
        val baseType = if (lv >= 4 && species.base == "grass") "sand" else species.base.ifBlank { "grass" }

        if (lv <= 0) {
            val ctx = RenderCtx(
                doc = doc, cx = cx, cy = cy, scale = scale,
                sleep = sleep, sil = false, breath = breath, blinkScaleY = blinkScaleY, eggWobbleDeg = eggWobbleDeg,
                tokens = mapOf(
                    "@body" to bodyTint,
                    "@bodyTint72" to blendWithWhite(bodyTint, 0.72f),
                    "@bodyTint35" to blendWithWhite(bodyTint, 0.35f),
                    "@bodyTint12" to blendWithWhite(bodyTint, 0.12f)
                ),
                paint = paint
            )
            drawPedestalSpec(canvas, baseType, ctx)
            drawShape(canvas, PetSpecStore.ShapeSpec.Group("egg", tpl.stage0Shapes), ctx)
            return
        }

        if (lv == 1) {
            val ctx = RenderCtx(
                doc = doc, cx = cx, cy = cy, scale = scale,
                sleep = sleep, sil = false, breath = breath, blinkScaleY = blinkScaleY, eggWobbleDeg = 0f,
                tokens = mapOf("@body" to bodyTint),
                paint = paint
            )
            drawPedestalSpec(canvas, baseType, ctx)
            val s1 = tpl.stage1
            val clipBounds = RectF(
                ctx.tx(s1.clip.left), ctx.ty(s1.clip.top),
                ctx.tx(s1.clip.right), ctx.ty(s1.clip.bottom)
            )
            val inner = ArrayList<PetSpecStore.ShapeSpec>()
            inner.addAll(s1.ears[species.ear] ?: emptyList())
            if (species.ahoge > 0) inner.addAll(s1.ahoge)
            inner.addAll(s1.head)
            inner.addAll(s1.mouths[species.mouth] ?: s1.mouths["smile"] ?: emptyList())

            val anim = ctx.doc.animations.breathe
            canvas.save()
            if (breath > 0f) {
                val sx = 1f + breath * (anim.maxScaleX - 1f)
                val sy = 1f - breath * (1f - anim.minScaleY)
                canvas.scale(sx, sy, ctx.tx(anim.originX), ctx.ty(anim.originY))
            }
            canvas.save()
            canvas.clipRect(clipBounds)
            drawShapes(canvas, inner, ctx)
            canvas.restore()
            drawShapes(canvas, s1.shell, ctx)
            canvas.restore()
            return
        }

        val clampedLv = lv.coerceAtMost(5)
        val rule = tpl.stageRules[clampedLv]
        val evolvedSp = when {
            rule?.overrideAcc != null || rule?.wingToTail != null -> species.copy(
                acc = rule.overrideAcc ?: species.acc,
                tail = if (rule.wingToTail != null && species.tail == "wing") rule.wingToTail else species.tail
            )
            rule?.crownIfNoAcc == true && (species.acc.isBlank() || species.acc == "none") -> species.copy(acc = "crown")
            else -> species
        }.copy(base = baseType)
        val halo = tpl.haloByLv[clampedLv]
        drawSpecies(
            canvas = canvas,
            sp = evolvedSp,
            cx = cx,
            cy = cy,
            scale = scale,
            silhouette = false,
            drawBase = true,
            sleep = sleep,
            breath = breath,
            blinkScaleY = blinkScaleY,
            halo = halo,
            stageScale = rule?.scale,
            stageAnchorX = rule?.anchorX ?: 110f,
            stageAnchorY = rule?.anchorY ?: 174f,
            paint = paint
        )
    }

    private fun drawPedestalSpec(canvas: Canvas, base: String, ctx: RenderCtx) {
        val ped = ctx.doc.pedestals
        val pal = ped.palettes[base] ?: ped.palettes["grass"] ?: return
        val pedCtx = ctx.copy(
            tokens = ctx.tokens + mapOf(
                "@pedFill" to PetArtCanvas.parseColorSafe(pal.fill),
                "@pedStroke" to PetArtCanvas.parseColorSafe(pal.stroke),
                "@pedDot" to PetArtCanvas.parseColorSafe(pal.dot)
            )
        )
        drawShapes(canvas, ped.shapes, pedCtx)
    }

    private fun drawHalo(canvas: Canvas, halo: PetSpecStore.HaloSpec, ctx: RenderCtx) {
        val paint = ctx.paint
        paint.shader = null
        paint.style = Paint.Style.FILL
        paint.color = applyOpacity(PetArtCanvas.parseColorSafe(halo.fill), halo.opacity)
        canvas.drawCircle(ctx.tx(halo.cx), ctx.ty(halo.cy), halo.r * ctx.scale, paint)
    }

    private fun drawShapes(canvas: Canvas, shapes: List<PetSpecStore.ShapeSpec>, ctx: RenderCtx) {
        for (s in shapes) drawShape(canvas, s, ctx)
    }

    private fun drawShape(canvas: Canvas, sh: PetSpecStore.ShapeSpec, ctx: RenderCtx) {
        when (sh.whenCond) {
            "awake" -> if (ctx.sleep) return
            "normal" -> if (ctx.sil) return
            "sil" -> if (!ctx.sil) return
        }
        val p = ctx.paint
        when (sh) {
            is PetSpecStore.ShapeSpec.Group -> {
                canvas.save()
                if (!ctx.sil) {
                    if (sh.anim == "breathe" && ctx.breath > 0f) {
                        val b = ctx.doc.animations.breathe
                        val sx = 1f + ctx.breath * (b.maxScaleX - 1f)
                        val sy = 1f - ctx.breath * (1f - b.minScaleY)
                        canvas.scale(sx, sy, ctx.tx(b.originX), ctx.ty(b.originY))
                    } else if (sh.anim == "egg" && ctx.eggWobbleDeg != 0f) {
                        val ew = ctx.doc.animations.eggWobble
                        canvas.rotate(ctx.eggWobbleDeg, ctx.tx(ew.pivotX), ctx.ty(ew.pivotY))
                    }
                }
                drawShapes(canvas, sh.children, ctx)
                canvas.restore()
            }
            is PetSpecStore.ShapeSpec.Eyes -> {
                PetArtCanvas.drawEyes(canvas, ctx.tx(sh.x1), ctx.tx(sh.x2), ctx.ty(sh.y), ctx.scale, ctx.sleep, p, ctx.blinkScaleY)
            }
            is PetSpecStore.ShapeSpec.TextLabel -> {
                p.shader = null
                p.style = Paint.Style.FILL
                p.color = resolveColor(sh.fill, 1f, ctx) ?: 0xFF8D8578.toInt()
                p.textSize = sh.size * ctx.scale
                p.textAlign = Paint.Align.CENTER
                p.isFakeBoldText = true
                canvas.drawText(sh.text, ctx.tx(sh.x), ctx.ty(sh.y), p)
                p.isFakeBoldText = false
            }
            is PetSpecStore.ShapeSpec.Ellipse -> {
                val scx = ctx.tx(sh.cx)
                val scy = ctx.ty(sh.cy)
                val srx = sh.rx * ctx.scale
                val sry = sh.ry * ctx.scale
                val rect = RectF(scx - srx, scy - sry, scx + srx, scy + sry)
                canvas.save()
                if (sh.rotDeg != null) canvas.rotate(sh.rotDeg, ctx.tx(sh.rotX), ctx.ty(sh.rotY))
                drawPaintGeometry(sh.fill, sh.silFill, sh.stroke, sh.sw, null, sh.opacity, rect, ctx) {
                    canvas.drawOval(rect, p)
                }
                canvas.restore()
            }
            is PetSpecStore.ShapeSpec.Circle -> {
                val scx = ctx.tx(sh.cx)
                val scy = ctx.ty(sh.cy)
                val sr = sh.r * ctx.scale
                val rect = RectF(scx - sr, scy - sr, scx + sr, scy + sr)
                drawPaintGeometry(sh.fill, sh.silFill, sh.stroke, sh.sw, null, sh.opacity, rect, ctx) {
                    canvas.drawCircle(scx, scy, sr, p)
                }
            }
            is PetSpecStore.ShapeSpec.RoundRect -> {
                val left = ctx.tx(sh.x)
                val top = ctx.ty(sh.y)
                val rect = RectF(left, top, left + sh.w * ctx.scale, top + sh.h * ctx.scale)
                val rr = sh.rx * ctx.scale
                drawPaintGeometry(sh.fill, sh.silFill, sh.stroke, sh.sw, null, sh.opacity, rect, ctx) {
                    canvas.drawRoundRect(rect, rr, rr, p)
                }
            }
            is PetSpecStore.ShapeSpec.VectorPath -> {
                val path = SvgPathParser.buildAndroidPath(sh.cmds, ctx.ox, ctx.oy, ctx.cx, ctx.cy, ctx.scale)
                val scrBounds = RectF(
                    ctx.tx(sh.bounds.left), ctx.ty(sh.bounds.top),
                    ctx.tx(sh.bounds.right), ctx.ty(sh.bounds.bottom)
                )
                canvas.save()
                if (sh.rotDeg != null) canvas.rotate(sh.rotDeg, ctx.tx(sh.rotX), ctx.ty(sh.rotY))
                drawPaintGeometry(sh.fill, sh.silFill, sh.stroke, sh.sw, sh.cap, sh.opacity, scrBounds, ctx) {
                    canvas.drawPath(path, p)
                }
                canvas.restore()
            }
        }
    }

    private inline fun drawPaintGeometry(
        fillRaw: String?,
        silFillRaw: String?,
        strokeRaw: String?,
        sw: Float,
        cap: String?,
        opacity: Float,
        bounds: RectF,
        ctx: RenderCtx,
        drawFn: () -> Unit
    ) {
        val p = ctx.paint
        val fillSpec = if (ctx.sil && silFillRaw != null) silFillRaw else fillRaw
        if (fillSpec != null) {
            p.style = Paint.Style.FILL
            if (fillSpec.startsWith("grad:")) {
                val grad = buildRadialShader(fillSpec.substring(5), bounds, ctx)
                if (grad != null) {
                    p.color = applyOpacity(Color.BLACK, opacity)
                    p.shader = grad
                } else {
                    p.shader = null
                    p.color = applyOpacity(Color.GRAY, opacity)
                }
            } else {
                p.shader = null
                p.color = resolveColor(fillSpec, opacity, ctx) ?: Color.TRANSPARENT
            }
            drawFn()
            p.shader = null
        }
        if (strokeRaw != null) {
            p.shader = null
            p.style = Paint.Style.STROKE
            p.color = resolveColor(strokeRaw, opacity, ctx) ?: Color.TRANSPARENT
            p.strokeWidth = sw * ctx.scale
            p.strokeCap = if (cap == "round") Paint.Cap.ROUND else Paint.Cap.BUTT
            drawFn()
            p.style = Paint.Style.FILL
        }
    }

    private fun buildRadialShader(name: String, bounds: RectF, ctx: RenderCtx): RadialGradient? {
        val spec = ctx.doc.gradients[name] ?: return null
        val bw = max(1f, bounds.width())
        val bh = max(1f, bounds.height())
        val gcx = bounds.left + bw * spec.cx
        val gcy = bounds.top + bh * spec.cy
        val gr = max(1f, max(bw, bh) * spec.r)
        val colors = IntArray(spec.stops.size)
        val positions = FloatArray(spec.stops.size)
        for (i in spec.stops.indices) {
            val st = spec.stops[i]
            val baseCol = resolveColor(st.colorToken, st.alpha, ctx) ?: Color.TRANSPARENT
            colors[i] = baseCol
            positions[i] = st.offset
        }
        return RadialGradient(gcx, gcy, gr, colors, positions, Shader.TileMode.CLAMP)
    }

    private fun resolveColor(raw: String?, opacity: Float, ctx: RenderCtx): Int? {
        if (raw == null) return null
        val silColor = 0xFFBFB8AC.toInt()
        val resolved = when {
            raw.startsWith("@") -> {
                if (ctx.sil && (raw == "@body" || raw == "@acc" || raw.startsWith("@bodyTint"))) silColor
                else ctx.tokens[raw] ?: silColor
            }
            ctx.sil && raw == "#bfb8ac" -> silColor
            else -> PetArtCanvas.parseColorSafe(raw)
        }
        return applyOpacity(resolved, opacity)
    }

    private fun applyOpacity(color: Int, opacity: Float): Int {
        if (opacity >= 0.999f) return color
        val baseA = Color.alpha(color)
        val newA = (baseA * opacity.coerceIn(0f, 1f)).roundToInt().coerceIn(0, 255)
        return Color.argb(newA, Color.red(color), Color.green(color), Color.blue(color))
    }

    private fun blendWithWhite(color: Int, whiteWeight: Float): Int {
        val w = whiteWeight.coerceIn(0f, 1f)
        val r = (Color.red(color) * (1f - w) + 255f * w).toInt().coerceIn(0, 255)
        val g = (Color.green(color) * (1f - w) + 255f * w).toInt().coerceIn(0, 255)
        val b = (Color.blue(color) * (1f - w) + 255f * w).toInt().coerceIn(0, 255)
        return Color.rgb(r, g, b)
    }
}
