let tmpInterval = null;

function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function refreshPanelRects(panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs) {
    const pid = "p" + panelIdx;
    const span = document.getElementById(pid + 'rects');
    let innerhtml = "";
    for (let j = 0; j < maxImagePanelTextAreas; j++) {
        var ptext = document.getElementById(pid + "t" + j + langs[0]).value;
        for (let ptk = 1; ptk < langs.length; ptk++) {
            const el = document.getElementById(pid + "t" + j + langs[ptk]);
            if (el == document.activeElement && el.value && el.value.length) {
                ptext = el.value;
                break;
            }
        }
        const trX = parseInt(document.getElementById(pid + "t" + j + "rx").value);
        const trY = parseInt(document.getElementById(pid + "t" + j + "ry").value);
        const trW = parseInt(document.getElementById(pid + "t" + j + "rw").value);
        const trH = parseInt(document.getElementById(pid + "t" + j + "rh").value);
        if ((!(isNaN(trW) || isNaN(trH) || isNaN(trX) || isNaN(trY))) && (trW > 0) && (trH > 0)) {
            innerhtml += "<div class='panelrect col" + j + "' style='left:" + (trX - pOffX) + "px; top:" + (trY - pOffY) + "px; width: " + trW + "px; height: " + trH + "px;'>";
            innerhtml += "<svg viewbox='0 0 " + trW + " " + trH + "'><text x='0' y='0'>";
            for (let line of ptext.split('\n')) {
                if ((!line) || line.length == 0)
                    line = '&nbsp;';
                innerhtml += "<tspan dy='" + AppProjGenPanelSvgTextPerLineDy + "' x='0'>" + line
                    .replace(/\s/g, "&nbsp;")
                    .replace(/<b>/g, "<tspan class='b'>")
                    .replace(/<u>/g, "<tspan class='u'>")
                    .replace(/<i>/g, "<tspan class='i'>")
                    .replace(/<\/b>/g, "</tspan>")
                    .replace(/<\/u>/g, "</tspan>")
                    .replace(/<\/i>/g, "</tspan>")
                    + "</tspan>"
            }
            innerhtml += "</text></svg></div>"
        }
    }
    span.innerHTML = innerhtml;
}

function onPanelClick(pid) {
    const cfgbox = document.getElementById(pid + "cfg");
    cfgbox.style.display = (cfgbox.style.display == 'none') ? 'block' : 'none';
}

function onPanelAuxClick(evt, panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs) {
    const pid = "p" + panelIdx;
    if (evt.target != evt.currentTarget)
        return;
    const cfgbox = document.getElementById(pid + "cfg");
    cfgbox.style.display = 'block';
    let ridx = undefined, trX, trY, trW, trH;
    for (ridx = 0; ridx < maxImagePanelTextAreas; ridx++) {
        trX = parseInt(document.getElementById(pid + "t" + ridx + "rx").value);
        trY = parseInt(document.getElementById(pid + "t" + ridx + "ry").value);
        trW = parseInt(document.getElementById(pid + "t" + ridx + "rw").value);
        trH = parseInt(document.getElementById(pid + "t" + ridx + "rh").value);
        if (((isNaN(trX) || (trX == 0)) && (isNaN(trY) || (trY == 0))) || ((isNaN(trW) || (trW == 0)) && (isNaN(trH) || (trH == 0))))
            break;
        else if (ridx == (maxImagePanelTextAreas - 1)) {
            ridx = undefined;
            break;
        }
    }
    if (ridx != undefined) {
        if ((isNaN(trX) || (trX == 0)) && (isNaN(trY) || (trY == 0))) {
            document.getElementById(pid + "t" + ridx + "rx").value = (pOffX + evt.offsetX).toString();
            document.getElementById(pid + "t" + ridx + "ry").value = (pOffY + evt.offsetY).toString();
        } else if ((isNaN(trW) || (trW == 0)) && (isNaN(trH) || (trH == 0))) {
            let rw = (pOffX + evt.offsetX) - trX, rh = (pOffY + evt.offsetY) - trY;
            if (rw < 0) {
                rw = -rw;
                document.getElementById(pid + "t" + ridx + "rx").value = (pOffX + evt.offsetX).toString();
            }
            if (rh < 0) {
                rh = -rh;
                document.getElementById(pid + "t" + ridx + "ry").value = (pOffY + evt.offsetY).toString();
            }
            document.getElementById(pid + "t" + ridx + "rw").value = rw.toString();
            document.getElementById(pid + "t" + ridx + "rh").value = rh.toString();
        }
        refreshPanelRects(panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs);
    }
}
