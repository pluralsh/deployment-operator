import requests
from bs4 import BeautifulSoup
import yaml

# Scrape Ansible and Python version compatibility
url = "https://docs.ansible.com/ansible/latest/reference_appendices/release_and_maintenance.html"
response = requests.get(url)
soup = BeautifulSoup(response.content, "html.parser")

version_pairs = []

# Find the tables for Ansible Community and Core versions
tables = soup.find_all("table")

# Parse the Ansible Community versions and corresponding Core versions
community_core_versions = {}
community_table = tables[1]  # The second table
community_rows = community_table.find_all("tr")[1:]

for row in community_rows:
    columns = row.find_all("td")
    if len(columns) >= 3:
        community_version = columns[0].text.split()[
            0
        ]  # Extract the version number
        status = columns[1].text.strip()
        core_version = columns[2].text.strip()
        if status != "In development (unreleased)":
            community_core_versions[community_version] = core_version

# Parse the Core versions and corresponding Python versions
core_python_versions = {}
core_table = tables[2]  # The third table
core_rows = core_table.find_all("tr")[1:]

for row in core_rows:
    columns = row.find_all("td")
    if len(columns) >= 5:
        core_version = columns[0].text.strip()
        python_versions_control_node = columns[3].text.strip().split(", ")
        last_python_version = python_versions_control_node[-1].split()[-1]
        core_python_versions[core_version] = [last_python_version]

# print("Core versions and their corresponding Python versions:")
# for core_version, python_versions in core_python_versions.items():
#     print(f"{core_version} : {python_versions}")

# Combine the data to get Community and their corresponding Python versions
for community_version, core_version in community_core_versions.items():
    python_versions = core_python_versions.get(core_version, [])
    for python_version in python_versions:
        version_pairs.append(
            {
                "ansible": community_version,
                "python": python_version,
                "tag": community_version.split(".")[0]
                + "."
                + community_version.split(".")[1],
            }
        )

print("Combined version pairs:")
for pair in version_pairs:
    print(pair)

# Exit for testing
exit()

# Load and update the GitHub Action YAML
github_action_path = "../.github/workflows/publish-harness.yaml"

with open(github_action_path, "r") as file:
    github_action = yaml.safe_load(file)

# Update the Ansible job strategy matrix
github_action["jobs"]["publish-harness-ansible"]["strategy"]["matrix"][
    "versions"
] = version_pairs

# Write the updated GitHub Action YAML back to the file
with open(github_action_path, "w") as file:
    yaml.safe_dump(github_action, file)

print("GitHub Action YAML updated successfully!")
