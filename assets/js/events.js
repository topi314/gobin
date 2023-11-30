document.getElementById("files").addEventListener("change", (e) => {
    const file = window.gobin.files[e.target.value];

    document.getElementById("code-edit").innerText = file.content;
    document.getElementById("code-view").innerHTML = file.formatted;
    document.getElementById("code-edit-count").innerText = file.content.length;
    document.getElementById("language").value = file.language;
})

document.getElementById("files").addEventListener("dblclick", (e) => {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = true;
    e.target.focus();
});

document.getElementById("files").addEventListener("focusout", (e) => {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = false;
});

document.getElementById("files").addEventListener("keypress", (e) => {
    if (e.key === "Enter") {
        e.preventDefault();
        if (e.target.contentEditable) {
            e.target.blur();
        }
    }
});

document.getElementById("file-add").addEventListener("click", (e) => {

});

document.getElementById("content").addEventListener("keydown", (e) => {
    if (e.key !== "Tab" || e.shiftKey) {
        return;
    }
    e.preventDefault();

    const start = e.target.selectionStart;
    const end = e.target.selectionEnd;
    e.target.value = e.target.value.substring(0, start) + "\t" + e.target.value.substring(end);
    e.target.selectionStart = e.target.selectionEnd = start + 1;
});

document.getElementById("content").addEventListener("input", (e) => {
    const count = e.target.value.length;

    const countElement = document.querySelector("#code-edit-count");
    countElement.innerHTML = count
    const maxElement = document.querySelector("#code-edit-max");
    if (!maxElement) return;
    if (count > maxElement.innerHTML) {
        countElement.classList.add("error");
    } else {
        countElement.classList.remove("error");
    }
});

document.getElementById("version").addEventListener("change", (e) => {

});

document.getElementById("style").addEventListener("change", (e) => {
    const style = e.target.value;
    const theme = e.target.options.item(e.target.selectedIndex).dataset.theme;
    setCookie("style", style);
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.replace(theme === "dark" ? "light" : "dark", theme);
    const themeCssElement = document.getElementById("theme-css");

    const href = new URL(themeCssElement.href);
    href.searchParams.set("style", style);
    themeCssElement.href = href.toString();
});

document.getElementById("language").addEventListener("change", (e) => {

});
