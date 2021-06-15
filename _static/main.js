let tmpInterval = null;

function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function refreshPanelRects(panelIdx, pOffX, pOffY, maxImagePanelTextAreas, langs) {
    try {
        const pid = "p" + panelIdx;
        const span = document.getElementById(pid + 'rects');
        let innerhtml = "";
        for (let j = 0; j < maxImagePanelTextAreas; j++) {
            var ptext = document.getElementById(pid + "t" + j + langs[0]).value;
            for (let ptk = 1; ptk < langs.length; ptk++) {
                const el = document.getElementById(pid + "t" + j + langs[ptk]);
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
                innerhtml += "<div class='panelrect col" + j + "' style='left:" + (trx0 - pOffX) + "px; top:" + (try0 - pOffY) + "px; width: " + width + "px; height: " + height + "px;'>";
                innerhtml += "<svg viewbox='0 0 " + width + " " + height + "'><text x='0' y='0'>";
                for (const line of ptext.split('\n')) {
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
    } catch (e) {
        alert(e);
    }
}

function toggle(id) {
    const elem = document.getElementById(id);
    elem.style.display = (elem.style.display == 'none') ? 'block' : 'none';
}
