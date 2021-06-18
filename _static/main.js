let pLangIdx = 0;

function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function refreshAllPanelRects(numPanels, langIdx, langName) {
    pLangIdx = langIdx;
    for (let i = 0; i < numPanels; i++)
        document.getElementById('p' + i + 't0' + langName).dispatchEvent(new Event("change"));
}

function refreshPanelRects(panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs, px1cm) {
    const pid = "p" + panelIdx;
    const span = document.getElementById(pid + 'rects');
    let innerhtml = "";
    const pxfont = parseInt(px1cm * svgTxtFontSizeCmA4);
    const pxline = parseInt(px1cm * svgTxtPerLineDyCmA4);

    for (let i = 0; i < maxImagePanelTextAreas; i++) {
        var ptext = document.getElementById(pid + "t" + i + langs[pLangIdx]).value;
        for (let langidx = 0; langidx < langs.length; langidx++) {
            const el = document.getElementById(pid + "t" + i + langs[langidx]);
            if ((el == document.activeElement && el.value && el.value.length) || (langidx == 0 && !ptext)) {
                ptext = el.value;
                break;
            }
        }
        const trX = parseInt(document.getElementById(pid + "t" + i + "rx").value);
        const trY = parseInt(document.getElementById(pid + "t" + i + "ry").value);
        const trW = parseInt(document.getElementById(pid + "t" + i + "rw").value);
        const trH = parseInt(document.getElementById(pid + "t" + i + "rh").value);
        if ((!(isNaN(trW) || isNaN(trH) || isNaN(trX) || isNaN(trY))) && (trW > 0) && (trH > 0)) {
            innerhtml += "<div class='panelrect col" + i + "' style='left:" + (trX - pOffX) + "px; top:" + (trY - pOffY) + "px; width: " + trW + "px; height: " + trH + "px;'>";
            innerhtml += "<svg viewbox='0 0 " + trW + " " + trH + "'><text x='0' y='0'>";
            for (let line of ptext.split('\n')) {
                if ((!line) || line.length == 0)
                    line = '&nbsp;';
                innerhtml += "<tspan style='font-size: " + pxfont + "px' dy='" + pxline + "' x='0'>" + line
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
    if (span.innerHTML != innerhtml)
        span.innerHTML = innerhtml;
}

function onPanelClick(pid) {
    const cfgbox = document.getElementById(pid + "cfg");
    cfgbox.style.display = (cfgbox.style.display == 'none') ? 'block' : 'none';
}

function onPanelAuxClick(evt, panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs, zoomDiv) {
    const pid = "p" + panelIdx;
    if (evt.target != evt.currentTarget)
        return;
    const ex = parseInt(evt.offsetX * zoomDiv), ey = parseInt(evt.offsetY * zoomDiv);
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
            document.getElementById(pid + "t" + ridx + "rx").value = (pOffX + ex).toString();
            document.getElementById(pid + "t" + ridx + "ry").value = (pOffY + ey).toString();
        } else if ((isNaN(trW) || (trW == 0)) && (isNaN(trH) || (trH == 0))) {
            let rw = (pOffX + ex) - trX, rh = (pOffY + ey) - trY;
            if (rw < 0) {
                rw = -rw;
                document.getElementById(pid + "t" + ridx + "rx").value = (pOffX + ex).toString();
            }
            if (rh < 0) {
                rh = -rh;
                document.getElementById(pid + "t" + ridx + "ry").value = (pOffY + ey).toString();
            }
            document.getElementById(pid + "t" + ridx + "rw").value = rw.toString();
            document.getElementById(pid + "t" + ridx + "rh").value = rh.toString();
        }
        document.getElementById(pid + "t" + ridx + langs[pLangIdx]).dispatchEvent(new Event("change"));
    }
}

function toggleScanOpt(curScanDev) {
    var divs = document.getElementsByClassName("scandevopts");
    if (divs && divs.length)
        for (let i = 0; i < divs.length; i++)
            if (divs[i] && divs[i].style)
                divs[i].style.display = (divs[i].id == 'scandevopts_' + curScanDev) ? 'block' : 'none';
}
