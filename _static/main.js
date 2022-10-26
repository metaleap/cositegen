let pLangIdx = 0;

function doPostBack(name) {
    $.main_focus_id.value = name;
    $.main_form.submit();
}

function refreshAllPanelRects(numPanels, langIdx, langName) {
    pLangIdx = langIdx;
    for (let i = 0; i < numPanels; i++)
        document.getElementById('p' + i + 't0' + langName).dispatchEvent(new Event("change"));
}

function onDualIntTextInputKeyDown(evt) {
    if (!(evt && evt.target && evt.code && evt.code.startsWith('Arrow') && !evt.altKey))
        return;
    evt.preventDefault();
    evt.cancelBubble = true;
    let txt = evt.target.value.split(',');
    let tx = (txt && txt.length) ? txt[0] : '', ty = (txt && txt.length > 1) ? txt[1] : '';
    if (tx == '' && ty == '')
        [tx, ty] = ['0', '0'];
    let x = parseInt(tx), y = parseInt(ty);
    if ((!isNaN(x)) && !isNaN(y)) {
        let delta = (evt.ctrlKey && evt.shiftKey) ? 1000 : (evt.ctrlKey ? 1 : (evt.shiftKey ? 100 : 10));
        switch (evt.code) {
            case "ArrowUp":
                y -= delta;
                break;
            case "ArrowLeft":
                x -= delta;
                break;
            case "ArrowRight":
                x += delta;
                break;
            case "ArrowDown":
                y += delta;
                break;
        }
        if ((txt = x + ',' + y) != evt.target.value) {
            evt.target.value = txt;
            evt.target.dispatchEvent(new Event("change"));
        }
    }
}

function refreshPanelRects(panelIdx, pOffX, pOffY, pWidth, pHeight, langs, px1cm, panelSvgTextClsBoxPoly, panelSvgTextBoxPolyStrokeWidthCm, tspanSubTagStyles) {
    const pid = "p" + panelIdx;
    let innerhtml = "";
    const pxfont = parseInt(px1cm * svgTxtFontSizeCmA4);
    const pxline = parseInt(px1cm * svgTxtPerLineDyCmA4);

    let divshtml = "";
    innerhtml += "<div class='panelrect panelrectbordered'>";
    innerhtml += "<svg viewbox='0 0 " + pWidth + " " + pHeight + "'><rect x='0' y='0' width='" + pWidth + "px' height='" + pHeight + "px' fill-opacity='0' stroke-width='" + parseInt(px1cm * 0.22) + "px' stroke='#000000'/>";
    for (let i = 0; i < numImagePanelTextAreas; i++) {
        const trXy = document.getElementById(pid + "t" + i + "rxy").value.split(',');
        const trX = parseInt((trXy && trXy.length >= 2) ? trXy[0] : "");
        const trY = parseInt((trXy && trXy.length >= 2) ? trXy[1] : "");
        const trWh = document.getElementById(pid + "t" + i + "rwh").value.split(',');
        const trW = parseInt((trWh && trWh.length >= 2) ? trWh[0] : "");
        const trH = parseInt((trWh && trWh.length >= 2) ? trWh[1] : "");
        const trPxy = document.getElementById(pid + "t" + i + "rpxy").value.split(',');
        const trPx = parseInt((trPxy && trPxy.length >= 2) ? trPxy[0] : "");
        const trPy = parseInt((trPxy && trPxy.length >= 2) ? trPxy[1] : "");
        if ((!(isNaN(trW) || isNaN(trH) || isNaN(trX) || isNaN(trY))) && (trW > 0) && (trH > 0)) {
            const borderandfill = !(isNaN(trPx) || isNaN(trPy));
            let ptext = document.getElementById(pid + "t" + i + langs[pLangIdx]).value.trimEnd();
            for (let langidx = 0; langidx < langs.length; langidx++) {
                const el = document.getElementById(pid + "t" + i + langs[langidx]);
                if ((el == document.activeElement && el.value && el.value.length) || (langidx == 0 && !ptext)) {
                    ptext = el.value;
                    break;
                }
            }

            divshtml += "<div class='panelrect col" + ((ptext && ptext.trim().length && borderandfill) ? "" : i) + "' style='left:" + (trX - pOffX) + "px; top:" + (trY - pOffY) + "px; width: " + trW + "px; height: " + trH + "px;'></div>";

            const rw = trW, rh = trH, rx = trX - pOffX, ry = trY - pOffY;
            if (borderandfill) {
                const rpx = trPx - pOffX, rpy = trPy - pOffY;
                const mmh = px1cm * panelSvgTextBoxPolyStrokeWidthCm, cmh = px1cm * 0.5;
                const pl = (rx + mmh), pr = ((rx + rw) - mmh), pt = (ry + mmh), pb = ((ry + rh) - mmh);
                let poly = [pl + ',' + pt, pr + ',' + pt, pr + ',' + pb, pl + ',' + pb];
                if (!((trPx == 0) && (trPy == 0))) { // "speech-text" pointing somewhere?
                    const dx = Math.abs(rpx - (rx + (rw * 0.5))), dy = Math.abs(rpy - (ry + (rh * 0.5)));
                    const isr = rpx > (rx + (rw * 0.5)), isl = !isr,
                        isb = rpy > (ry + (rh * 0.5)), ist = !isb,
                        isbr = isb && isr && dy > dx,
                        isbl = isb && isl && dy > dx,
                        istr = ist && isr && dy > dx,
                        istl = ist && isl && dy > dx,
                        isrb = isr && isb && dx > dy && !isbr,
                        islb = isl && isb && dx > dy,
                        isrt = isr && ist && dx > dy,
                        islt = isl && ist && dx > dy,
                        dst = rpx + ',' + rpy; // coords to point towards
                    if (isbl || islb)
                        poly = arrIns(poly, 3, [(pl + cmh) + ',' + pb, dst]);
                    else if (isbr || isrb)
                        poly = arrIns(poly, 3, [dst, (pr - cmh) + ',' + pb]);
                    else if (istr)
                        poly = arrIns(poly, 1, [(pr - cmh) + ',' + pt, dst]);
                    else if (istl)
                        poly = arrIns(poly, 1, [dst, (pl + cmh) + ',' + pt]);
                    else if (isrt)
                        poly = arrIns(poly, 2, [dst, pr + ',' + (pt + cmh)]);
                    else if (islt)
                        poly = arrIns(poly, 4, [pl + ',' + (pt + cmh), dst]);
                    else if (islb)
                        poly = arrIns(poly, 4, [dst, pl + ',' + (pb - cmh)]);
                }
                innerhtml += "<polygon points='" + poly.join(' ') + "' class='" + panelSvgTextClsBoxPoly + "' stroke-width='" + mmh + "px'/>";
            }

            innerhtml += "<svg x='" + rx + "' y='" + ry + "'><text x='0' y='0' style='font-size: " + pxfont + "px' transform='" + document.getElementById(pid + "t" + i + "_transform").value.replace(/\n/g, " ").trim() + "'>";
            innerhtml += "<tspan style='" + document.getElementById(pid + "t" + i + "_style").value.replace(/\n/g, " ").trim() + "'>";
            for (let line of ptext.split('\n')) {
                if ((!line) || line.length == 0)
                    line = '&nbsp;';
                else {
                    line = line.replace(/\s/g, "&nbsp;");
                    if (tspanSubTagStyles)
                        for (const k in tspanSubTagStyles) {
                            line = line
                                .replace("<" + k + ">", "<tspan style='" + tspanSubTagStyles[k] + "'>")
                                .replace("</" + k + ">", "</tspan>");
                        }
                }
                innerhtml += "<tspan dy='" + (("_storytitle" == document.getElementById(pid + "t" + i + "_style").value) ? (1.23 * pxline) : pxline) + "' x='" + (borderandfill ? (px1cm * 0.44) : 0) + "'>"
                    + line
                        .replace(/<b>/g, "<tspan font-weight='bold'>")
                        .replace(/<u>/g, "<tspan text-decoration='underline'>")
                        .replace(/<i>/g, "<tspan font-style='italic'>")
                        .replace(/<\/b>/g, "</tspan>")
                        .replace(/<\/u>/g, "</tspan>")
                        .replace(/<\/i>/g, "</tspan>")
                    + "</tspan>";
            }
            innerhtml += "</tspan></text></svg>";
        }
    }
    innerhtml += "</svg></div>" + divshtml

    const span = document.getElementById(pid + 'rects');
    if (span && span.innerHTML != innerhtml)
        span.innerHTML = innerhtml;
}

function arrIns(arr, index, items) {
    let ret = [];
    for (let i = 0; i < index; i++)
        ret.push(arr[i]);
    for (let i = 0; i < items.length; i++)
        ret.push(items[i]);
    for (let i = index; i < arr.length; i++)
        ret.push(arr[i]);
    return ret;
}

function onPanelClick(pid) {
    const cfgbox = document.getElementById(pid + "cfg");
    cfgbox.style.display = (cfgbox.style.display == 'none') ? 'block' : 'none';
}

function onPanelAuxClick(evt, panelIdx, pOffX, pOffY, langs, zoomDiv) {
    const pid = "p" + panelIdx;
    const ex = parseInt(parseFloat(evt.offsetX) * zoomDiv), ey = parseInt(parseFloat(evt.offsetY) * zoomDiv);
    const cfgbox = document.getElementById(pid + "cfg");
    cfgbox.style.display = 'block';
    let ridx = undefined, trXy, trX, trY, trWh, trW, trH;
    for (ridx = 0; ridx < numImagePanelTextAreas; ridx++) {
        trXy = document.getElementById(pid + "t" + ridx + "rxy").value.split(',');
        trX = parseInt((trXy && trXy.length >= 2) ? trXy[0] : "");
        trY = parseInt((trXy && trXy.length >= 2) ? trXy[1] : "");
        trWh = document.getElementById(pid + "t" + ridx + "rwh").value.split(',');
        trW = parseInt((trWh && trWh.length >= 2) ? trWh[0] : "");
        trH = parseInt((trWh && trWh.length >= 2) ? trWh[1] : "");
        if (((isNaN(trX) || (trX == 0)) && (isNaN(trY) || (trY == 0))) || ((isNaN(trW) || (trW == 0)) && (isNaN(trH) || (trH == 0))))
            break;
        else if (ridx == (numImagePanelTextAreas - 1)) {
            ridx = undefined;
            break;
        }
    }
    if (ridx != undefined) {
        if ((isNaN(trX) || (trX == 0)) && (isNaN(trY) || (trY == 0))) {
            document.getElementById(pid + "t" + ridx + "rxy").value = (pOffX + ex) + "," + (pOffY + ey);
        } else if ((isNaN(trW) || (trW == 0)) && (isNaN(trH) || (trH == 0))) {
            let rw = (pOffX + ex) - trX, rh = (pOffY + ey) - trY;
            if (rw < 0) {
                rw = -rw;
                document.getElementById(pid + "t" + ridx + "rxy").value = (pOffX + ex) + "," + trY;
            }
            if (rh < 0) {
                rh = -rh;
                document.getElementById(pid + "t" + ridx + "rxy").value = trX + "," + (pOffY + ey);
            }
            document.getElementById(pid + "t" + ridx + "rwh").value = rw + "," + rh;
        }
        document.getElementById(pid + "t" + ridx + langs[pLangIdx]).dispatchEvent(new Event("change"));
    }
}

function toggleScanOptsPane(curScanDev) {
    var divs = document.getElementsByClassName("scandevopts");
    if (divs && divs.length)
        for (let i = 0; i < divs.length; i++)
            if (divs[i] && divs[i].style)
                divs[i].style.display = (divs[i].id == 'scandevopts_' + curScanDev) ? 'block' : 'none';
}

function kickOffScanJob() {
    const txt = $.sheetname;
    txt.value = txt.value.trim();
    if (!(txt.value && txt.value.length)) {
        txt.focus();
        alert("Sheet name missing but required.");
        return;
    }
    const btn = $.scanbtn;
    btn.disabled = 'disabled';
    btn.innerText = 'Wait...';
    const hid = $.scannow;
    hid.value = '1';
    doPostBack('');
}

function addBwtPreviewLinks(sheetVerSrcFilePath) {
    $.previewbwtlinks.innerHTML = '';
    let nums = $.previewbwt.value.split(',');
    if (!(nums && nums.length)) {
        return;
    }
    for (let i = 0; i < nums.length; i++) {
        nums[i] = parseInt(nums[i]);
        if (isNaN(nums[i]) || nums[i] < 1 || nums[i] > 255)
            return;
    }

    let html = '';
    for (let i = 0; i < nums.length; i++) {
        const imgsrc = sheetVerSrcFilePath + "/" + nums[i];
        html += "<a id='previewbwtlink" + i + "' href='" + imgsrc + "' target='" + imgsrc.replace(/\//g, '_') + "'>&nbsp;" + nums[i] + "&nbsp;</a>"
    }
    $.previewbwtlinks.innerHTML = html;
    $.previewbwtlink0.focus()
}

function txtPrev(idx) {
    const prevs = document.getElementsByClassName("txtprev"),
        id = idx ? ('txtprev' + idx) : '';
    for (let i = 0; i < prevs.length; i++)
        prevs[i].style.display = (prevs[i].id == id) ? 'block' : 'none';
}

function txtImp() {
    const idx = window.txtimpsel.value;
    if (idx && confirm("Sure?")) {
        doPostBack('txtimpsel')
    }
}
