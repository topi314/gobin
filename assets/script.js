hljs.listLanguages().forEach((language) => {
    const option = document.createElement("option");
    option.value = language;
    option.innerText = language;
    document.querySelector("#language").appendChild(option);
});

document.addEventListener("DOMContentLoaded", () => {
    const key = window.location.pathname === "/" ? "" : window.location.pathname.slice(1);
    let newState;
    let url;
    if (key) {
        const content = document.querySelector("#code-show").innerText
        newState = {key: key, mode: "view", content: content};
        url = `/${key}`;
    } else {
        newState = {key: "", mode: "edit", content: ""};
        url = "/";
    }
    window.history.replaceState(newState, "", url);

    const style = localStorage.getItem("stylePreference") || "github-dark.min.css"
    setStyle(style);

    updatePage(newState);
    updatePageButtons(newState);
});

window.addEventListener("popstate", (event) => {
    updatePage(event.state);
    updatePageButtons(event.state);
});

document.addEventListener("keydown", (event) => {
    if (!event.ctrlKey) return;

    switch (event.key) {
        case "s":
            event.preventDefault();
            if (document.querySelector("#save").disabled) return;
            document.querySelector("#save").click();
            break;
        case "n":
            event.preventDefault();
            if (document.querySelector("#new").disabled) return;
            document.querySelector("#new").click();
            break;
        case "e":
            event.preventDefault();
            if (document.querySelector("#edit").disabled) return;
            document.querySelector("#edit").click();
            break;
        case "d":
            event.preventDefault();
            if (document.querySelector("#delete").disabled) return;
            const deleteConfirm = confirm("Are you sure you want to delete this document? This action cannot be undone.")
            if (!deleteConfirm) return;
            document.querySelector("#delete").click();
            break;
    }
})

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const {key} = getState();
    const newState = {key: key, mode: "edit", content: event.target.value};
    window.history.replaceState(newState, "", `/${key}`);
    updatePageButtons(newState);
})

document.querySelector("#new").addEventListener("click", () => {
    window.open("/", "_blank").focus();
})

document.querySelector("#edit").addEventListener("click", async () => {
    if (document.querySelector("#edit").disabled) return;

    const {key, content} = getState();
    let newState;
    let url;
    if (getUpdateToken(key) === "") {
        newState = {key: "", mode: "edit", content: content};
        url = "/";
    } else {
        newState = {key: key, mode: "edit", content: content};
        url = `/${key}`;
    }
    window.history.pushState(newState, "", url);

    updatePage(newState);
    updatePageButtons(newState);
})

document.querySelector("#save").addEventListener("click", async () => {
    if (document.querySelector("#save").disabled) return;

    const {key, mode, content} = getState()
    if (mode !== "edit") return;
    const updateToken = getUpdateToken(key);
    const saveButton = document.querySelector("#save");
    saveButton.classList.add("loading");

    let response;
    if (key && updateToken) {
        response = await fetch(`/documents/${key}`, {
            method: "PATCH", body: content, headers: {
                Authorization: updateToken
            }
        });
    } else {
        response = await fetch("/documents", {
            method: "POST", body: content
        });
    }
    saveButton.classList.remove("loading");

    if (!response.ok) {
        showErrorPopup(response.message);
        console.error("error saving document:", response);
        return;
    }

    const body = await response.json();
    setUpdateToken(body.key, body.update_token);

    const newState = {key: body.key, mode: "view", content: content};
    window.history.pushState(newState, "", `/${body.key}`);
    updatePage(newState);
});

document.querySelector("#delete").addEventListener("click", async () => {
    if (document.querySelector("#delete").disabled) return;

    const {key} = getState();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        return;
    }
    let deleteButton = document.querySelector("#delete");
    deleteButton.classList.add("loading");

    let response = await fetch(`/documents/${key}`, {
        method: "DELETE", headers: {
            Authorization: updateToken
        }
    });
    deleteButton.classList.remove("loading");

    if (!response.ok) {
        showErrorPopup(response.message)
        console.error("error deleting document:", response);
        return;
    }
    deleteUpdateToken();
    const newState = {key: "", mode: "edit", content: ""};
    window.history.pushState(newState, "", "/");
    updatePage(newState);
})

document.querySelector("#copy").addEventListener("click", async () => {
    if (document.querySelector("#copy").disabled) return;

    const {content} = getState();
    if (!content) return;
    await navigator.clipboard.writeText(content);
})

document.querySelector("#raw").addEventListener("click", async () => {
    if (document.querySelector("#raw").disabled) return;

    const {key} = getState();
    if (!key) return;
    window.open(`/raw/${key}`, "_blank").focus();
})

document.querySelector("#language").addEventListener("change", (event) => {
    highlightCode(event.target.value);
});

document.querySelector("#style").addEventListener("change", (event) => {
    setStyle(event.target.value);
});

function showErrorPopup(message) {
    const popup = document.getElementById("error-popup");
    popup.style.display = "block";
    popup.innerText = message || "Something went wrong.";
    setTimeout(() => popup.style.display = "none", 5000);
}


function getState() {
    return window.history.state;
}

function getUpdateToken(key) {
    const documents = localStorage.getItem("documents")
    if (!documents) return ""
    const updateToken = JSON.parse(documents)[key]
    if (!updateToken) return ""

    return updateToken
}

function setUpdateToken(key, updateToken) {
    let documents = localStorage.getItem("documents")
    if (!documents) {
        documents = "{}"
    }
    const parsedDocuments = JSON.parse(documents)
    parsedDocuments[key] = updateToken
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
}

function deleteUpdateToken() {
    const {key} = getState();
    const documents = localStorage.getItem("documents");
    if (!documents) return;
    const parsedDocuments = JSON.parse(documents);
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments));
}

function updatePageButtons(state) {
    const {key, mode, content} = state;
    const updateToken = getUpdateToken(key);

    if (mode === "edit" && content && (!key || key && updateToken)) {
        document.querySelector("#save").disabled = false;
    } else {
        document.querySelector("#save").disabled = true;
    }

    if (mode === "edit" || !content) {
        document.querySelector("#new").disabled = true;
    } else {
        document.querySelector("#new").disabled = false;
    }

    if (updateToken) {
        document.querySelector("#delete").disabled = false;
        document.querySelector("#edit").disabled = false;
    } else {
        document.querySelector("#delete").disabled = true;
        document.querySelector("#edit").disabled = true;
    }

    if (key && mode === "view") {
        document.querySelector("#copy").disabled = false;
        document.querySelector("#raw").disabled = false;
    } else if (key) {
        document.querySelector("#copy").disabled = true;
        document.querySelector("#raw").disabled = true;
    }
}

function updatePage(state) {
    const {key, mode, content} = state;

    // update page title
    if (key) {
        document.title = `gobin - ${key}`;
    } else {
        document.title = "gobin";
    }

    if (mode === "view") {
        document.querySelector("#delete").disabled = false;
        document.querySelector("#save").disabled = true;
        document.querySelector("#edit").disabled = false;
        document.querySelector("#copy").disabled = false;
        document.querySelector("#raw").disabled = false;
        document.querySelector("#code-edit").style.display = "none";
        document.querySelector("#code-show").innerText = content;
        document.querySelector("#code").style.display = "block";
        highlightCode();
    } else if (mode === "edit") {
        document.querySelector("#delete").disabled = true;
        document.querySelector("#save").disabled = true;
        document.querySelector("#edit").disabled = true;
        document.querySelector("#copy").disabled = true;
        document.querySelector("#raw").disabled = true;
        document.querySelector("#code-show").innerText = "";
        document.querySelector("#code").style.display = "none";
        const codeEditElement = document.querySelector("#code-edit");
        codeEditElement.value = content;
        codeEditElement.style.display = "block";
    }
}

function setStyle(style) {
    localStorage.setItem("stylePreference", style)
    document.querySelector(`link[title="Highlight.js Style"]`).href = "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.7.0/styles/" + style;
    document.querySelector("#style").value = style;
}

function highlightCode(language = undefined) {
    const {content} = getState();
    let result;
    if (language && language !== "auto") {
        result = hljs.highlight(content, {
            language: language, ignoreIllegals: true
        });
    } else {
        result = hljs.highlightAuto(content);
    }
    if (result.language === undefined) {
        result.language = "plaintext";
    }

    const codeShowElement = document.querySelector("#code-show");
    codeShowElement.innerHTML = result.value;
    codeShowElement.className = "hljs language-" + result.language;

    document.querySelector("#language").value = result.language;

    if (result.value) {
        hljs.initLineNumbersOnLoad({singleLine: true});
    }
}