function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function refreshPanelRects(panelIdx, pOffX, pOffY, maxImagePanelAreas, panelTextKinds) {
    try {
        const pid = "p" + panelIdx;
        const span = document.getElementById(pid + 'rects');
        span.innerHTML = "";
        for (let j = 0; j < maxImagePanelAreas; j++) {
            var ptext = document.getElementById(pid + "t" + j + panelTextKinds[0]).value;
            for (let ptk = 1; ptk < panelTextKinds.length; ptk++) {
                const el = document.getElementById(pid + "t" + j + panelTextKinds[ptk]);
                if (el == document.activeElement) {
                    ptext = el.value;
                    break;
                }
            }
            const trx0 = parseInt(document.getElementById(pid + "t" + j + "rx0").value);
            const trx1 = parseInt(document.getElementById(pid + "t" + j + "rx1").value);
            const try0 = parseInt(document.getElementById(pid + "t" + j + "ry0").value);
            const try1 = parseInt(document.getElementById(pid + "t" + j + "ry1").value);
            const width = trx1 - trx0;
            const height = try1 - try0;
            if ((!(isNaN(width) || isNaN(height))) && width > 0 && height > 0) {
                span.innerHTML += "<div class='panelrect col" + j + "' style='left:" + (trx0 - pOffX) + "px; top:" + (try0 - pOffY) + "px; width: " + width + "px; height: " + height + "px;'>" + ptext + "</div>";
            }
        }
    } catch (e) {
        alert(e);
    }
}

function toggle(id) {
    const elem = document.getElementById(id);
    elem.style.display = (elem.style.display == 'none') ? 'block' : 'none';
}
