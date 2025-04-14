document.addEventListener('DOMContentLoaded', function() {
    // Add click event listener to the document
    document.addEventListener('click', function(event) {
        // Check if the clicked element or its parent has data-copy-url attribute
        let target = event.target;

        // Navigate up to 3 levels to find button (in case SVG or path was clicked)
        for (let i = 0; i < 3; i++) {
            if (!target) break;

            if (target.hasAttribute && target.hasAttribute('data-copy-url')) {
                // Get the URL to copy
                const url = target.getAttribute('data-copy-url');
                // Find the SVG element within the button
                const svg = target.querySelector('svg');

                // Copy to clipboard
                navigator.clipboard.writeText(url)
                    .then(() => {
                        // Store the original SVG path
                        const originalPath = svg.innerHTML
                        // Replace with checkmark SVG path
                        svg.innerHTML = '<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z"></path>';


                        // Reset title after 1 second
                        setTimeout(() => {
                            svg.innerHTML = originalPath;
                        }, 1000);
                    })
                    .catch(err => {
                        console.error('Failed to copy: ', err);
                        svg.innerHTML = originalPath;
                    });

                break;
            }

            target = target.parentElement;
        }
    });
});