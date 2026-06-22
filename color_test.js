// Create a simple red rectangle to test if colors work at all
const rect = figma.createRectangle();
rect.resize(200, 100);
rect.x = 500; rect.y = 400;
rect.fills = [{type: "SOLID", color: {r: 1, g: 0, b: 0, a: 1}, visible: true}];
rect.name = "test-red";
figma.currentPage.appendChild(rect);
"test rect created"
